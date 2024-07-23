package main

import (
	"context"
	"log"
	"sync"

	"github.com/google/go-github/v63/github"
	"github.com/noelukwa/indexer/internal/events"
)

func fetchCommits(ctx context.Context, client *github.Client, resolverChannel chan<- *github.RepositoryCommit, ev *events.NewIntent, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("fetching commits")
	opts := &github.CommitsListOptions{
		Since: ev.From,
		Until: ev.Until,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		log.Printf("fetching commits for %s/%s, from: %s to: %s", ev.RepoOwner, ev.RepoName, ev.From, ev.Until)
		commits, resp, err := client.Repositories.ListCommits(ctx, ev.RepoOwner, ev.RepoName, opts)
		if err != nil {
			log.Printf("Error fetching commits: %v", err)
			return
		}

		log.Printf("resp: %s", resp.Status)
		for _, commit := range commits {
			log.Println(commit.SHA)
			resolverChannel <- commit
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
}
