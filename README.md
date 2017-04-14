# `ecs-run-task`

[![Build Status](https://travis-ci.org/glassechidna/ecs-run-task.svg?branch=master)](https://travis-ci.org/glassechidna/ecs-run-task)

`ecs-run-task` is a cross-platform CLI tool to facilitate running ad-hoc tasks on the [AWS EC2 Container Service][aws-ecs] (ECS).

Occasionally the need arises to run tasks in our infrastructure that are not long-running services. Examples include database migrations, batch jobs, ad-hoc database queries and general spelunking. To enable this use case, ECS exposes the [RunTask][run-task] [functionality][run-task-docs].

[aws-ecs]: https://aws.amazon.com/ecs/
[run-task]: http://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html
[run-task-docs]: http://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_run_task.html

## Usage

```
$ ecs-run-task run --help
Run a pre-defined ECS Task on a given cluster with possible
command override.

Usage:
  ecs-run-task run [flags]

Flags:
      --cluster string
      --command string
      --container string
      --task-definition string

Global Flags:
      --config string    config file (default is $HOME/.ecs-run-task.yaml)
      --profile string   profile defined in ~/.aws/config
```
