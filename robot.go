package main

import (
	"errors"
	"fmt"
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	libconfig "github.com/opensourceways/community-robot-lib/config"
	"github.com/opensourceways/community-robot-lib/giteeclient"
	libplugin "github.com/opensourceways/community-robot-lib/giteeplugin"
	"github.com/sirupsen/logrus"
)

const (
	botName                 = "lifecycle"
	closeIssueMessage       = `this issue is closed by: ***@%s***.`
	reopenIssueMessage      = "this issue is opened by: ***@%s***."
	closePullRequestMessage = `this pull request is closed by: ***@%s***.`
	optionFailureMessage    = `***@%s*** you can't %s a %s unless you are the author of it or a collaborator.`
)

var (
	reopenRe = regexp.MustCompile(`(?mi)^/reopen\s*$`)
	closeRe  = regexp.MustCompile(`(?mi)^/close\s*$`)
)

type iClient interface {
	CreatePRComment(owner, repo string, number int32, comment string) error
	CreateIssueComment(owner, repo string, number string, comment string) error
	IsCollaborator(owner, repo, login string) (bool, error)
	CloseIssue(owner, repo string, number string) error
	ClosePR(owner, repo string, number int32) error
	ReopenIssue(owner, repo string, number string) error
}

func newRobot(cli iClient) *robot {
	return &robot{cli: cli}
}

type robot struct {
	cli iClient
}

func (bot *robot) NewPluginConfig() libconfig.PluginConfig {
	return &configuration{}
}

func (bot *robot) getConfig(cfg libconfig.PluginConfig) (*configuration, error) {
	if c, ok := cfg.(*configuration); ok {
		return c, nil
	}
	return nil, errors.New("can't convert to configuration")
}

func (bot *robot) RegisterEventHandler(p libplugin.HandlerRegitster) {
	p.RegisterNoteEventHandler(bot.handleNoteEvent)
}

func (bot *robot) handleNoteEvent(e *sdk.NoteEvent, cfg libconfig.PluginConfig, log *logrus.Entry) error {
	ne := giteeclient.NewNoteEventWrapper(e)

	if !ne.IsCreatingCommentEvent() {
		log.Debug("Event is not a creation of a comment for PR or issue, skipping.")
		return nil
	}
	config, err := bot.getConfig(cfg)
	if err != nil {
		return err
	}

	org, repo := ne.GetOrgRep()
	if configFor := config.configFor(org, repo); configFor == nil {
		log.Debug("don't care this event because can't get configuration by org and repo")
		return nil
	}

	if ne.IsPullRequest() {
		return bot.handlePullRequest(giteeclient.NewPRNoteEvent(e), log)
	}

	if ne.IsIssue() {
		return bot.handleIssue(giteeclient.NewIssueNoteEvent(e), log)
	}

	return nil
}

func (bot *robot) handlePullRequest(e giteeclient.PRNoteEvent, log *logrus.Entry) error {
	if !e.IsPROpen() || !closeRe.MatchString(e.GetComment()) {
		return nil
	}

	prInfo := e.GetPRInfo()
	commenter := e.GetCommenter()

	isCollaborator, err := bot.cli.IsCollaborator(prInfo.Org, prInfo.Repo, prInfo.Author)
	if err != nil {
		return err
	}

	if prInfo.Author != commenter && !isCollaborator {
		comment := fmt.Sprintf(optionFailureMessage, commenter, "close", "pull request")
		return bot.cli.CreatePRComment(prInfo.Org, prInfo.Repo, prInfo.Number, comment)
	}

	if err := bot.cli.ClosePR(prInfo.Org, prInfo.Repo, prInfo.Number); err != nil {
		return fmt.Errorf("Error closing PR: %v ", err)
	}

	return bot.cli.CreatePRComment(prInfo.Org, prInfo.Repo, prInfo.Number, fmt.Sprintf(closePullRequestMessage, commenter))
}

func (bot *robot) handleIssue(ne giteeclient.IssueNoteEvent, log *logrus.Entry) error {
	org, repo := ne.GetOrgRep()
	commenter := ne.GetCommenter()
	number := ne.GetIssueNumber()

	validate := func() (bool, error) {
		author := ne.GetIssueAuthor()
		isColl, err := bot.cli.IsCollaborator(org, repo, author)
		if err != nil {
			return false, err
		}
		return author == commenter || isColl, nil
	}

	if ne.IsIssueClosed() && reopenRe.MatchString(ne.GetComment()) {
		ok, err := validate()
		if err != nil {
			return err
		}

		if !ok {
			return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(optionFailureMessage, commenter, "open", "issue"))
		}

		if err := bot.cli.ReopenIssue(org, repo, number); err != nil {
			return err
		}

		return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(reopenIssueMessage, commenter))
	}

	if ne.IsIssueOpen() && closeRe.MatchString(ne.GetComment()) {
		ok, err := validate()
		if err != nil {
			return err
		}

		if !ok {
			return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(optionFailureMessage, commenter, "close", "issue"))
		}

		if err := bot.cli.CloseIssue(org, repo, number); err != nil {
			return err
		}

		return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(closeIssueMessage, commenter))
	}

	return nil
}
