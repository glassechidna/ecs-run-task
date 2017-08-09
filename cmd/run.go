// Copyright © 2017 Aidan Steele <aidan.steele@glassechidna.com.au>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"strings"
	"time"
	"log"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run ECS task",
	Long: `Run a pre-defined ECS Task on a given cluster with possible
command override.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
		sessOpts := session.Options{
			SharedConfigState: session.SharedConfigEnable,
			AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		}

		profile := cmd.Flag("profile").Value.String()
		if len(profile) > 0 {
			sessOpts.Profile = profile
		}

		sess, _ := session.NewSessionWithOptions(sessOpts)

		cluster := cmd.Flag("cluster").Value.String()
		taskDefinition := cmd.Flag("task-definition").Value.String()
		container := cmd.Flag("container").Value.String()
		command := cmd.Flag("command").Value.String()
		run(sess, taskDefinition, cluster, command, container)
	},
}

type CloudWatchLogConfig struct {
	Group string
	StreamPrefix string
}


func run(sess *session.Session, taskDefinition, cluster, command, container string) {
	client := ecs.New(sess)

	input := ecs.RunTaskInput{
		TaskDefinition: aws.String(taskDefinition),
		Cluster: aws.String(cluster),
		Overrides: &ecs.TaskOverride{
			ContainerOverrides: []*ecs.ContainerOverride{
				{
					Name: aws.String(container),
					Command: []*string{
						aws.String("bash"),
						aws.String("-c"),
						aws.String(fmt.Sprintf("bash -c '%v'; EXITCODE=$?; echo \"TASK FINISHED, EXITCODE: $EXITCODE\"", command)),
					},
				},
			},
		},
	}

	output, err := client.RunTask(&input)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	logConfig, err := cloudWatchConfig(client, taskDefinition, container)

	taskArn := *output.Tasks[0].TaskArn
	taskId := taskArn[len(taskArn)-36:]
	stream := logConfig.StreamPrefix + "/" + container+ "/" + taskId

	log.Printf("started task id: %s\n", taskId)

	cwClient := cloudwatchlogs.New(sess)

Loop:
	for {
		// TODO: also need to poll ecs task for errors, e.g. failure to pull image, essential container stopped, etc
		
		time.Sleep(1 * time.Second)

		describeTasksOutput, _ := client.DescribeTasks(&ecs.DescribeTasksInput{
			Tasks: aws.StringSlice([]string{taskId}),
			Cluster: aws.String(cluster),
		})

		//out, _ := json.MarshalIndent(describeTasksOutput, "", "  ")
		//os.Stderr.Write(out)

		taskOutput := describeTasksOutput.Tasks[0]
		container := taskOutput.Containers[0]

		if *taskOutput.LastStatus == "STOPPED" &&
			*taskOutput.DesiredStatus == "STOPPED" &&
			container.ExitCode == nil {
			log.Fatalf("Task %s stopped (%s) because:\n\n\t%s: %s\n", taskId, *taskOutput.StoppedReason, *container.Name, *container.Reason)
		}

		streamsOutput, _ := cwClient.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logConfig.Group),
			LogStreamNamePrefix: aws.String(stream),
		})

		for idx := range streamsOutput.LogStreams {
			streamThing := streamsOutput.LogStreams[idx]
			if *streamThing.LogStreamName == stream {
				break Loop
			}
		}
	}

	cwParams := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName: aws.String(logConfig.Group),
		LogStreamName: aws.String(stream),
		StartFromHead: aws.Bool(true),
	}
	cwClient.GetLogEventsPages(cwParams, func(page *cloudwatchlogs.GetLogEventsOutput, lastPage bool) bool {
		for idx := range page.Events {
			event := page.Events[idx]
			if strings.HasPrefix(*event.Message, "TASK FINISHED, EXITCODE:") {
				return false
			}
			fmt.Println(*event.Message)
		}
		return true
	})

	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func cloudWatchConfig(client *ecs.ECS, taskDefinition string, containerName string) (*CloudWatchLogConfig, error) {
	taskDefInput := ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	taskDefOutput, err := client.DescribeTaskDefinition(&taskDefInput)

	if err != nil {
		return nil, err
	}

	for idx := range taskDefOutput.TaskDefinition.ContainerDefinitions {
		containerDef := taskDefOutput.TaskDefinition.ContainerDefinitions[idx]
		if *containerDef.Name == containerName {
			group, ok := containerDef.LogConfiguration.Options["awslogs-group"]

			if !ok {
				log.Panicf("Container %s does not use awslogs logging driver", containerName)
			}

			streamPrefix, ok := containerDef.LogConfiguration.Options["awslogs-stream-prefix"]
			// can use http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_FilterLogEvents.html
			// to determine which stream if we don't know (e.g. stream prefix hasn't been set)
			// there doesn't seem to be a container arn -> container id mapping: https://github.com/aws/amazon-ecs-agent/issues/258

			config := CloudWatchLogConfig{
				Group: *group,
				StreamPrefix: *streamPrefix,
			}
			return &config, nil
		}
	}

	return nil, nil // TODO create an error type
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().String("cluster", "", "")
	runCmd.Flags().String("task-definition", "", "")
	runCmd.Flags().String("container", "", "")
	runCmd.Flags().String("command", "", "")
}
