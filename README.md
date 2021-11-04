### robot-gitee-lifecycle

[中文README](README_zh_CN.md)

#### Overview

lifcycle robot provides `/close` and `/reopen` commands for gitee platform users to close issues and Pull Requests and to reopen closed issues.

#### Command

| command | example | description                    | who can use                                                  |
| ------- | ------- | ------------------------------ | ------------------------------------------------------------ |
| /close  | /close  | Close a Pull Request or Issue. | Authors and repository collaborators can trigger such commands. |
| /reopen | /reopen | Reopen an Issue.               | Authors and repository collaborators can trigger such commands. |

#### Configuration

```yaml
config_items:
  - repos:  # list of repositories to be managed by robot
     -  owner/repo
     -  owner1
    excluded_repos: #Robot manages the list of repositories to be excluded
     - owner1/repo1
```

