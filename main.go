package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/afjoseph/clone_your_org/projectpath"
	"github.com/afjoseph/commongo/print"
	"github.com/afjoseph/commongo/util"
	"github.com/google/go-github/v33/github"
	"golang.org/x/oauth2"
)

var (
	GitAccessTokenFlag   = flag.String("git_access_token", "", "REQUIRED: Git OAuth2 access token")
	OrganizationNameFlag = flag.String("target_organization_name", "", "REQUIRED: Name of the GH organization to backup")
	BackupDirPathFlag    = flag.String("backup_dir", "", "OPTIONAL: backup directory. If you don't supply one, it'll be created in the root of the project")
)

const maxElementsPerPage = 100000000

func getGitClient(token string) (*github.Client, context.Context, error) {
	if len(token) == 0 {
		return nil, nil, print.Errorf("nil access token")
	}
	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)))
	return client, ctx, nil
}

// cloneRepo uses 'client' and 'ctx' to mirror clone a 'repo'
func cloneRepo(client *github.Client, ctx context.Context,
	backupDirPath string, repo *github.Repository) error {
	print.DebugFunc()

	targetDir := filepath.Join(backupDirPath, fmt.Sprintf("%s.git", *repo.Name))
	print.Debugf("Cloning %s to %s...\n", *repo.SSHURL, targetDir)
	_, _, _, err := util.Exec("", "git clone --mirror --recurse-submodules -j8 %s %s",
		*repo.SSHURL, targetDir)
	if err != nil {
		return err
	}
	return nil
}

// backupRepoIssuesAndPRs uses 'client' and 'ctx' to loop over issues in 'repo'
// and write them to a file
//
// XXX An "issue" is basically a "pull request" in GitHub's API. This function
// iterates over all issues which will effectively give you all issues+PRs.
// That being said, this function **won't** tell you which branch a PR is
// merging. I don't think that's a very important detail for a backup.
//
// XXX This function **disregards** attachments: if there's an attachment,
// you'll just see the GH link, but it won't explicitly download it.
func backupRepoIssuesAndPRs(client *github.Client, ctx context.Context,
	backupDirPath string, repo *github.Repository) error {
	print.DebugFunc()

	targetDir := filepath.Join(backupDirPath, fmt.Sprintf("%s__issues", *repo.Name))
	os.MkdirAll(targetDir, os.ModePerm)
	issues, _, err := client.Issues.ListByRepo(ctx,
		*repo.Owner.Login, *repo.Name, &github.IssueListByRepoOptions{
			State:       "all",
			ListOptions: github.ListOptions{PerPage: maxElementsPerPage},
		})
	if err != nil {
		return err
	}
	print.Debugf("Backing up %d issues for repo %s to %s\n", len(issues), *repo.Name, targetDir)
	for _, issue := range issues {
		// XXX I think 6 digits is a pretty decent limit
		issueFilePath := filepath.Join(targetDir, fmt.Sprintf("%06d.md", *issue.Number))
		print.Debugf("Backing up issue #%d to %s\n", *issue.Number, issueFilePath)
		fd, err := os.Create(issueFilePath)
		if err != nil {
			return err
		}
		fd.WriteString(fmt.Sprintf("* Issue #%d: %s\r\n", *issue.Number, *issue.Title))
		fd.WriteString(fmt.Sprintf("* Created at: %v\r\n", *issue.CreatedAt))
		fd.WriteString(fmt.Sprintf("* Author: %s\r\n", *issue.User.Login))
		if issue.Labels != nil {
			fd.WriteString("* Labels: ")
			for i, label := range issue.Labels {
				if i > 0 {
					fd.WriteString(", ")
				}
				fd.WriteString(*label.Name)
			}
			fd.WriteString("\r\n")
		}
		if issue.ClosedBy != nil {
			fd.WriteString(fmt.Sprintf("* Closed at: %s\r\n", *issue.ClosedAt))
			fd.WriteString(fmt.Sprintf("* Closed by: %s\r\n", *issue.ClosedBy.Login))
		}
		fd.WriteString("\r\n")
		if issue.Body != nil {
			fd.WriteString("## Description\r\n\r\n")
			fd.WriteString(fmt.Sprintf("%s\r\n\r\n", *issue.Body))
		}

		comments, _, err := client.Issues.ListComments(ctx, *repo.Owner.Login,
			*repo.Name, *issue.Number, nil)
		if err != nil {
			return err
		}
		print.Debugf("Found %d comments for issue #%d\n", len(comments), *issue.Number)
		for i, comment := range comments {
			print.Debugf("Comment by [%s]: at [%v]\n", *comment.User.Login, *comment.CreatedAt)

			// XXX Start counting from 1, not 0
			fd.WriteString(fmt.Sprintf("## Comment #%d\r\n\r\n", i+1))
			fd.WriteString(fmt.Sprintf("* By %s\r\n", *comment.User.Login))
			fd.WriteString(fmt.Sprintf("* At %v\r\n", *comment.CreatedAt))
			fd.WriteString(fmt.Sprintf("%s\r\n\r\n", *comment.Body))
		}
		err = fd.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func _main() error {
	print.SetLevel(print.LOG_DEBUG)

	// Parse flags
	// -----------
	flag.Parse()
	if len(*GitAccessTokenFlag) == 0 {
		return print.Errorf("nil git access token")
	}
	if len(*OrganizationNameFlag) == 0 {
		return print.Errorf("nil Organization")
	}
	var backupDirPath string
	// If BackupDirPathFlag is supplied, use it. Else, make one in the root of the project
	if len(*BackupDirPathFlag) != 0 {
		backupDirPath = util.ExpandPath(*BackupDirPathFlag)
	} else {
		backupDirPath = filepath.Join(projectpath.Root,
			fmt.Sprintf("backup__%s__%s",
				// yyMMdd_hhmmss
				time.Now().Format("060102_150405"),
				*OrganizationNameFlag),
		)
		err := util.SafeDelete(projectpath.Root, backupDirPath)
		if err != nil {
			return err
		}
	}
	print.Debugf("git_access_token: %+v, target_organization_name: %+v, backupDirPath: %+v\n",
		*GitAccessTokenFlag, *OrganizationNameFlag, backupDirPath)

	// Get Git client
	// -----------
	print.Debugf("Backing up %s organization to %s...\n",
		*OrganizationNameFlag, backupDirPath)
	client, ctx, err := getGitClient(*GitAccessTokenFlag)
	if err != nil {
		return err
	}

	// List Org repos and start the backup process
	// -----------
	repos, _, err := client.Repositories.ListByOrg(ctx, *OrganizationNameFlag, nil)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		print.Debugf("working with %s\n", *repo.Name)
		err = cloneRepo(client, ctx, backupDirPath, repo)
		if err != nil {
			return err
		}
		err = backupRepoIssuesAndPRs(client, ctx, backupDirPath, repo)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	err := _main()
	if err != nil {
		print.Warnln(err)
		os.Exit(1)
	}
}
