package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
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
		debug        bool
	)

	flag.StringVar(&logGroupName, "logGroup", "", "log group name")
	flag.StringVar(&region, "region", "", "region")
	flag.BoolVar(&debug, "debug", false, "output debug information")

	flag.Parse()

	if region == "" {
		usage("Missing region")
	}
	if logGroupName == "" {
		usage("Missing log group name")
	}

	sess := session.New(&aws.Config{Region: aws.String(region)})
	logEvent := &LogEvent{
		logMessages: make(chan string),
		sess:        sess,
		svc:         cloudwatchlogs.New(sess),
		sigs:        make(chan os.Signal, 1),
		debug:       debug,
	}

	signal.Notify(logEvent.sigs, syscall.SIGINT, syscall.SIGTERM)

	// handle signal
	go logEvent.handleSignals()

	// create log stream
	logStreamName, err := logEvent.createLogStream(logGroupName)
	if err != nil {
		panic(err)
	}

	logEvent.input = &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	}

	// capture stdin
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			out := scanner.Text()
			logEvent.logMessages <- out
		}

		if err := scanner.Err(); err != nil {
			log.Println(err)
		}
		close(logEvent.logMessages)
	}()

	logEvent.readLoop()
}

type LogEvent struct {
	logMessages chan string
	sess        *session.Session
	svc         *cloudwatchlogs.CloudWatchLogs
	sigs        chan os.Signal
	token       string
	input       *cloudwatchlogs.PutLogEventsInput
	debug       bool
}

func (l *LogEvent) writeLogEvent() string {
	if len(l.input.LogEvents) > 0 {
		result, err := l.svc.PutLogEvents(l.input)
		if err != nil {
			panic(err)
		}
		if l.debug {
			fmt.Printf("Wrote log event: %+v\n", result)
		}
		return aws.StringValue(result.NextSequenceToken)
	}
	return aws.StringValue(l.input.SequenceToken)
}

func (l *LogEvent) createLogStream(logGroupName string) (string, error) {
	logStreamName := uuid.New().String()
	_, err := l.svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	})
	return logStreamName, err
}

func (l *LogEvent) handleSignals() {
	sig := <-l.sigs
	fmt.Printf("%s", sig)
	l.input.LogEvents = append(l.input.LogEvents, &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(fmt.Sprintf("%s", sig)),
		Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
	})
	_ = l.writeLogEvent()
	os.Exit(0)
}
func (l *LogEvent) readLoop() {
	start := time.Now()

	for elem := range l.logMessages {
		fmt.Println(elem)

		if l.token != "" {
			l.input.SequenceToken = aws.String(l.token)
		}
		if len(elem) > 0 {
			l.input.LogEvents = append(l.input.LogEvents, &cloudwatchlogs.InputLogEvent{
				Message:   aws.String(elem),
				Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
			})
		}
		if time.Since(start) > time.Duration(1*time.Second) {
			l.token = l.writeLogEvent()
			l.input.LogEvents = []*cloudwatchlogs.InputLogEvent{}
			start = time.Now()
		}
	}
	if len(l.input.LogEvents) > 0 {
		_ = l.writeLogEvent()
	}
}
