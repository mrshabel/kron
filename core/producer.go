package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const (
	DefaultCronPollInterval      = 10 * time.Second
	DefaultCrontabPath           = "./tabs/default.crontab"
	DefaultBrokerDeliveryTimeout = 30 * time.Second

	StaleEntriesTTL = 1 * time.Hour
)

// errors
var (
	ErrInvalidCronFormat     = errors.New("invalid cron expression format")
	ErrBrokerDeliveryTimeout = errors.New("broker delivery timeout")
)

// scan crontab for active jobs, send to kafka

type ProducerConfig struct {
	Topic                 string
	CrontabPath           string
	CronPollInterval      time.Duration
	BrokerDeliveryTimeout time.Duration
	logger                *slog.Logger
}

func (cfg *ProducerConfig) validate() error {
	if cfg.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	if cfg.CrontabPath == "" {
		cfg.CrontabPath = DefaultCrontabPath
	}
	if cfg.CronPollInterval <= 0 {
		cfg.CronPollInterval = DefaultCronPollInterval
	}
	if cfg.BrokerDeliveryTimeout <= 0 {
		cfg.BrokerDeliveryTimeout = DefaultBrokerDeliveryTimeout
	}
	// add logger if not passed
	if cfg.logger == nil {
		cfg.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return nil
}

type Producer struct {
	config       *ProducerConfig
	producer     *kafka.Producer
	Topic        string
	deliveryChan chan kafka.Event
	logger       *slog.Logger
	// jobID -> last schedule time
	scheduledJobs map[string]time.Time
	// mutex to lock cached scheduled jobs
	mu sync.RWMutex
	// cron schedule parser
	cronParser cron.Parser
}

// NewKronProducer instantiates a new instance of the producer. The caller should call the Shutdown method on server shutdown
func NewKronProducer(cfg *ProducerConfig) (*Producer, error) {
	// validate config and set defaults
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	p, err := NewProducer()
	if err != nil {
		return nil, err
	}

	producer := &Producer{producer: p, Topic: cfg.Topic, deliveryChan: make(chan kafka.Event, 1000), logger: cfg.logger, scheduledJobs: make(map[string]time.Time), cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)}
	producer.config = cfg

	return producer, nil
}

func (p *Producer) GetReadyJobs() ([]*Job, error) {
	// load file
	file, err := os.Open(p.config.CrontabPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// read file contents
	scanner := bufio.NewScanner(file)
	var jobs []*Job

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		// get valid lines
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		content := strings.SplitN(line, " ", 6)
		// skip invalid cron expressions without failing completely
		if len(content) < 6 {
			p.logger.Warn("invalid crontab entry", "line", lineNumber, "content", line)
			continue
		}
		expression, command := strings.Join(content[:5], " "), content[5]

		schedule, err := p.cronParser.Parse(expression)
		if err != nil {
			p.logger.Warn("failed to parse cron expression", "line", lineNumber, "content", line, "error", err)
			continue
		}
		now := time.Now()
		scheduledAt := schedule.Next(now)

		// check if job schedule time is within the next polling interval and schedule it
		if scheduledAt.After(now) && scheduledAt.Before(now.Add(p.config.CronPollInterval)) {
			jobKey := p.newJobKey(command, scheduledAt)
			if p.isDuplicate(jobKey, scheduledAt) {
				continue
			}

			// schedule job
			jobs = append(jobs, &Job{Command: command, ScheduledAt: scheduledAt})
			p.scheduleJob(jobKey, scheduledAt)
		}

		// check for scanner errors
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading crontab: %w", err)
		}

	}
	return jobs, nil
}

func (p *Producer) DispatchJob(job *Job) error {
	// create job payload
	jobID := uuid.New().String()
	job.ID = jobID

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	msg := &kafka.Message{
		Key:            []byte(jobID),
		Value:          payload,
		TopicPartition: kafka.TopicPartition{Topic: &p.Topic, Partition: kafka.PartitionAny},
	}

	if err := p.producer.Produce(msg, p.deliveryChan); err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	// wait for delivery report
	select {
	case <-time.After(p.config.BrokerDeliveryTimeout):
		return ErrBrokerDeliveryTimeout
	case e := <-p.deliveryChan:
		res := e.(*kafka.Message)
		if res.TopicPartition.Error != nil {
			return fmt.Errorf("failed to schedule job: %w", res.TopicPartition.Error)
		}
		p.logger.Info("job dispatched", "jobId", jobID, "scheduledAt", job.ScheduledAt, "partition", res.TopicPartition.Partition, "offset", res.TopicPartition.Offset)
	}

	return nil
}

func (p *Producer) Run(ctx context.Context) error {
	p.logger.Info("starting kron producer/scheduler")
	// poll jobs
	ticker := time.NewTicker(p.config.CronPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("shutting down producer")
			return ctx.Err()
		case <-ticker.C:
			jobs, err := p.GetReadyJobs()
			if err != nil {
				p.logger.Error("failed to get ready jobs", "error", err)
				continue
			}

			// track job count
			if len(jobs) > 0 {
				p.logger.Info("jobs retrieved", "count", len(jobs))
			}

			for _, job := range jobs {
				if err = p.DispatchJob(job); err != nil {
					p.logger.Error("failed to dispatch job", "id", job.ID, "command", job.Command, "error", err)
					continue
				}
			}
		}
	}
}

func (p *Producer) Shutdown() {
	// flush buffered contents
	p.producer.Flush(int((5 * time.Second).Seconds()))
	p.producer.Close()
	// close delivery channel
	close(p.deliveryChan)
	p.logger.Info("producer shutdown complete")
}

// helpers

// newJobKey creates a deterministic job key for the specified job
func (p *Producer) newJobKey(command string, scheduledAt time.Time) string {
	return fmt.Sprintf("%s:%v", command, scheduledAt.Unix())
}

// isDuplicate checks if the current job has been scheduled within the past window. (1 minute)
func (p *Producer) isDuplicate(key string, scheduledAt time.Time) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if lastScheduledTime, ok := p.scheduledJobs[key]; ok {
		return scheduledAt.Sub(lastScheduledTime) < 1*time.Minute
	}
	return false
}

// scheduleJob marks a job as scheduled
func (p *Producer) scheduleJob(key string, scheduledAt time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scheduledJobs[key] = scheduledAt

	// add to scheduled jobs and invalidate stale entries
	now := time.Now()
	for key, st := range p.scheduledJobs {
		if now.Sub(st) > time.Hour {
			delete(p.scheduledJobs, key)
		}
	}
}
