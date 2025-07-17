# KRON

A distributed cron system, based on Kafka for processing scheduled tasks as specified in a crontab across multiple nodes.

Native cron systems process tasks on a single node, creating a single point of failure. Kron improves this by scheduling tasks on producer node(s) and distributing to consumer nodes using Kafka.

## Setup

-   Start the kafka instance and create the necessary topics

```bash
make start-kafka

# create jobs topic with 2 partitions
make create-topic topic=jobs partitions=2
```

-   Start producer and consumer services

```bash
make start
```

-   Update the contents of the crontab file in `tabs/default.crontab`

## Considerations

Some technical decisions made in the development of Kron

-   Producer reads the contents of the crontab every minute for updates.
