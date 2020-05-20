package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/google/uuid"
)

func usage(str string) {
	fmt.Printf("Error: %s\n", str)
	flag.Usage()
	os.Exit(1)
}

func main() {
	var (
		logGroupName string
		region       string
	)

	flag.StringVar(&logGroupName, "logGroup", "", "log group name")
	flag.StringVar(&region, "region", "", "region")

	flag.Parse()

	if region == "" {
		usage("Missing region")
	}
	if logGroupName == "" {
		usage("Missing log group name")
	}

	logMessages := make(chan string)

	// initialize aws sdk
	sess := session.New(&aws.Config{Region: aws.String(region)})
	svc := cloudwatchlogs.New(sess)

	logStreamName, err := createLogStream(svc, logGroupName)
	if err != nil {
		panic(err)
	}

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			out := scanner.Text()
			logMessages <- out
		}

		if err := scanner.Err(); err != nil {
			log.Println(err)
		}
		close(logMessages)
	}()

	var token string
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	}

	start := time.Now()

	for elem := range logMessages {
		fmt.Println(elem)

		if token != "" {
			input.SequenceToken = aws.String(token)
		}
		input.LogEvents = append(input.LogEvents, &cloudwatchlogs.InputLogEvent{
			Message:   aws.String(elem),
			Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
		})
		if time.Since(start) > time.Duration(1*time.Second) {
			token = writeLogEvent(svc, input)
			input.LogEvents = []*cloudwatchlogs.InputLogEvent{}
			start = time.Now()
		}
	}
	if len(input.LogEvents) > 0 {
		_ = writeLogEvent(svc, input)
	}

	os.Exit(0)

}

func writeLogEvent(svc *cloudwatchlogs.CloudWatchLogs, input *cloudwatchlogs.PutLogEventsInput) string {
	result, err := svc.PutLogEvents(input)
	if err != nil {
		panic(err)
	}
	//fmt.Printf("Wrote log event: %+v\n", result)
	return aws.StringValue(result.NextSequenceToken)
}

func createLogStream(svc *cloudwatchlogs.CloudWatchLogs, logGroupName string) (string, error) {
	logStreamName := uuid.New().String()
	_, err := svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	})
	return logStreamName, err
}
