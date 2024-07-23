package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

func fetchCommits(ctx context.Context, client *github.Client, resolverChannel chan<- *github.RepositoryCommit, repo string, startDate, endDate time.Time, wg *sync.WaitGroup) {
	defer wg.Done()

	opts := &github.CommitsListOptions{
		Since: startDate,
		Until: endDate,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		commits, resp, err := client.Repositories.ListCommits(ctx, "owner", repo, opts)
		if err != nil {
			log.Printf("Error fetching commits: %v", err)
			return
		}

		for _, commit := range commits {
			resolverChannel <- commit
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
}

func createGitHubClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}
