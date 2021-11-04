### robot-gitee-lifecycle

#### 概述

lifcycle robot 为gitee平台用户提供了`/close`与`/reopen`指令来关闭issue和Pull Request以及重新打开已经关闭的issue。

#### 命令

| 命令    | 示例    | 描述                            | 谁能使用                           |
| ------- | ------- | ------------------------------- | ---------------------------------- |
| /close  | /close  | 关闭一个Pull Request或者Issue。 | 作者和仓库的协作者能触发这种命令。 |
| /reopen | /reopen | 重新打开一个Issue。             | 作者和仓库的协作者能触发这种命令。 |

#### 配置

```yaml
config_items:
  - repos:  #robot需管理的仓库列表
     -  owner/repo
     -  owner1
    excluded_repos: #robot 管理列表中需排除的仓库
     - owner1/repo1
```

