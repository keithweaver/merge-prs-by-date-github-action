package main

import (
	"log"
	"os"
	"time"
	_ "time/tzdata"
)

func main() {
	owner, repo, token := os.Getenv("REPO_OWNER"), os.Getenv("REPO"), os.Getenv("GITHUB_ACCESS_TOKEN")
	if owner == "" || repo == "" || token == "" {
		log.Fatal("REPO_OWNER, REPO and GITHUB_ACCESS_TOKEN are required")
	}
	location, method, err := configure()
	if err != nil {
		log.Fatal(err)
	}
	time.Local = location
	client := NewGitHubClient(token, owner, repo)
	client.mergeMethod = method
	if err := Run(client, time.Now().In(location), log.Printf); err != nil {
		log.Fatal(err)
	}
}
