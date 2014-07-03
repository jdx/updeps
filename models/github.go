package models

import (
	"fmt"
	"os"
	"time"

	"code.google.com/p/goauth2/oauth"

	"github.com/google/go-github/github"
)

func (c *Client) RefreshPackagesGithub() {
	packageChannel := make(chan Package)
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
	}
	githubClient := github.NewClient(t.Client())
	for i := 0; i < 10; i++ {
		go c.githubWorker(githubClient, packageChannel)
	}
	packages, err := c.AllPackages()
	if err != nil {
		panic(err)
	}
	for _, pkg := range packages {
		if pkg.GithubName != "" && pkg.GithubUpdatedAt.IsZero() {
			packageChannel <- pkg
		}
	}
}

func (c *Client) githubWorker(githubClient *github.Client, packages <-chan Package) {
	for pkg := range packages {
		repo, resp, err := githubClient.Repositories.Get(pkg.GithubOwner, pkg.GithubName)
		if resp.StatusCode == 403 {
			resp.Body.Close()
			rateLimitUntil := c.githubRateLimitedUntil(githubClient)
			fmt.Println("rate limited until", rateLimitUntil)
			time.Sleep(-1 * time.Since(rateLimitUntil))
			continue
		} else if resp.StatusCode == 404 {
			fmt.Println("404", pkg)
			continue
		}
		if err != nil {
                        fmt.Println("Error processing", pkg)
			panic(err)
		}
		pkg.GithubForks = *repo.ForksCount
		pkg.GithubStargazers = *repo.StargazersCount
		pkg.GithubUpdatedAt = time.Now()
		fmt.Println("updating package", pkg)
		if _, err := c.UpsertPackage(&pkg); err != nil {
			panic(err)
		}
	}
}

func (c *Client) githubRateLimitedUntil(githubClient *github.Client) time.Time {
	rateLimits, _, err := githubClient.RateLimits()
	if err != nil { panic(err) }
	return rateLimits.Core.Reset.Time
}
