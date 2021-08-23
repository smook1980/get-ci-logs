package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
)

func main() {
	buildId := flag.String("b", "", "CodeBuild Build ID")
	profile := flag.String("p", "", "AWS Profile")
	flag.Parse()

	if *buildId == "" {
		fmt.Fprintln(os.Stderr, "Build ID (-a ID) is required")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Maybe cache with this?
	// https://github.com/99designs/aws-vault
	// https://github.com/99designs/keyring
	// cache_dir = os.path.join(os.path.expanduser('~'), '.aws', 'cli', 'cache')
	// credential_provider.cache = botocore.credentials.JSONFileCache(cache_dir)

	cfg, err := config.LoadDefaultConfig(ctx, func(o *config.LoadOptions) error {
		if profile != nil && *profile != "" {
			o.SharedConfigProfile = *profile
		}

		o.AssumeRoleCredentialOptions = func(b *stscreds.AssumeRoleOptions) {
			b.TokenProvider = func() (string, error) {
				var v string
				fmt.Fprintf(os.Stderr, "Assume Role MFA token code: ")
				_, err := fmt.Scanln(&v)

				return v, err
			}
		}

		return nil
	})
	failOnError("Failed to load AWS Config", err)

	fmt.Fprintf(os.Stderr, "Pulling Logs for %s\n", *buildId)

	cb := codebuild.NewFromConfig(cfg)
	buildBatch, err := cb.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{Ids: []string{*buildId}})

	failOnError("Failed to fetch CodeBuild Build details.", err)

	build := buildBatch.Builds[0]
	cbLogs := build.Logs.CloudWatchLogs

	buidMatcher, _ := regexp.Compile("[^:]*:(.*)$")
	buid := buidMatcher.FindStringSubmatch(*build.Id)[1]
	logStream := fmt.Sprintf("%s/%s", *cbLogs.StreamName, buid)
	fmt.Fprintf(os.Stderr, "\n%s\n", logStream)

	cwl := cloudwatchlogs.NewFromConfig(cfg)
	cwOpts := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  cbLogs.GroupName,
		LogStreamName: aws.String(logStream),
		// StartTime:     aws.Int64(build.StartTime.Unix()),
		// EndTime:       aws.Int64(build.EndTime.Unix()),
		// Limit:         aws.Int32(100),
	}

	cwOut, err := cwl.GetLogEvents(ctx, cwOpts)
	failOnError("Failed to fetch CloudWatch Log Batch.", err)

	for _, e := range cwOut.Events {
		fmt.Print(*e.Message)
	}

	os.Exit(0)

}

func failOnError(msg string, err error) {
	if err == nil {
		return
	}

	fmt.Println(fmt.Errorf("%s\n %w", msg, err))
	os.Exit(1)
}
