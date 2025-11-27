package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
)

func dumpOutput(v any, err error) {
	if err != nil {
		log.Fatal(err)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	e.Encode(v)
}

func main() {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	apiKey := flag.String("api-key", "", "GetDX API key")
	apiURL := flag.String("api-url", "", "GetDX API URL")

	teamID := flag.String("team-id", "", "GetDX team ID")

	eventName := flag.String("event-name", "", "Event name")
	email := flag.String("email", "", "Email")
	githubUserName := flag.String("github-user-name", "", "GitHub user name")
	gitlabUserName := flag.String("gitlab-user-name", "", "GitLab user name")
	testData := flag.Bool("event-test-data", false, "The event is a test data")
	flag.Parse()

	client, err := dx.NewWebAPIClient(dx.WithWebAPIKey(*apiKey), dx.WithWebAPIURL(*apiURL))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	commands := map[string]func(){
		"list-teams": func() {
			dumpOutput(client.ListTeams())
		},
		"get-team-info": func() {
			dumpOutput(client.GetTeamInfo(*teamID))
		},
		"send-event": func() {
			sendEventArgs := []dx.EventArg{}
			if *eventName != "" {
				sendEventArgs = append(sendEventArgs, dx.WithEventName(*eventName))
			}
			if *email != "" {
				sendEventArgs = append(sendEventArgs, dx.WithEventEmail(*email))
			}
			if *githubUserName != "" {
				sendEventArgs = append(sendEventArgs, dx.WithEventGitHubUserName(*githubUserName))
			}
			if *gitlabUserName != "" {
				sendEventArgs = append(sendEventArgs, dx.WithEventGitLabUserName(*gitlabUserName))
			}
			if *testData {
				sendEventArgs = append(sendEventArgs, dx.WithEventTestData(*testData))
			}
			fi, _ := os.Stdin.Stat()
			if (fi.Mode() & os.ModeCharDevice) == 0 {
				decoder := json.NewDecoder(os.Stdin)
				event := map[string]any{}
				if err := decoder.Decode(&event); err != nil {
					log.Fatalf("Failed to parse event from stdin: %v", err)
				}
				sendEventArgs = append(sendEventArgs, dx.WithEventMetadata(event))
			}
			dumpOutput(client.TrackEvent(sendEventArgs...))
		},
	}
	for _, arg := range flag.Args() {
		command, ok := commands[arg]
		if !ok {
			log.Fatalf("Unknown command: %s", arg)
		}
		command()
	}
}
