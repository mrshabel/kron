# KRON

A distributed cron system, based on Kafka for processing scheduled tasks as specified in a crontab across multiple nodes.

Native cron systems process tasks on a single node, creating a single point of failure. Kron improves this by scheduling tasks on producer node(s) and distributing to consumer nodes using Kafka.

## Setup

-   Update the contents of the crontab file in `tabs/default.crontab`
-   Start the kafka instance and create the necessary topics

```bash
make start-kafka

# create jobs topic with 2 partitions for each cluster
make setup-topics
```

-   Start producer and consumer services

```bash
make start
```

## Considerations

Some technical decisions made in the development of Kron

-   Producer reads the contents of the crontab every minute for updates since the lowest time unit in the cron entry is a minute.
