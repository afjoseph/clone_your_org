# clone_your_org: Easy wrapper to clone your GitHub's organization repos, issues and PRs

## Dependencies

* Go version 1.6

## Usage

```
go run . \
  -git_access_token=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  -target_organization_name=twitter
  # Optionally, you can specify a directory to use with -backupDirPath. Else,
  one will be created at the root of the project
```

## Getting an OAuth2 GitHub token

* Go to https://github.com/settings/tokens
* Click "Generate New Token"
* You'll need the following permissions for this project to work: `read:discussion, read:org, read:user, repo`
  * Most probably you need **less** permissions, but I haven't tested it that granularly
