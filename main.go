package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func main() {
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	// load file
	file, err := os.Open("./tabs/crontab")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// read file contents
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		// get valid lines
		txt := scanner.Text()
		if strings.HasPrefix(txt, "#") || len(txt) == 0 {
			continue
		}

		content := strings.SplitN(txt, " ", 6)
		if len(content) < 6 {
			log.Fatal("invalid crontab entry: ", txt)
		}
		expr, command := strings.Join(content[:5], " "), content[5]
		log.Printf("parsed expression: %#v, command: %v", expr, command)

		sched, err := specParser.Parse(expr)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("next cron time", sched.Next(time.Now()))
	}
}
