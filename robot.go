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
	botName                   = "lifecycle"
	issueOptionFailureMessage = `***@%s*** you can't %s an issue unless you are the author of it or a collaborator.`
	prOptionFailureMessage    = `***@%s*** you can't %s a pull request unless you are the author of it or a collaborator.`
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
	if config.configFor(org, repo) == nil {
		log.Debug("ignore this event, because of no configuration for this repo.")
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

	v, err := bot.hasPermission(prInfo.Org, prInfo.Repo, commenter, prInfo.Author)
	if err != nil {
		return err
	}

	if !v {
		comment := fmt.Sprintf(prOptionFailureMessage, commenter, "close")
		return bot.cli.CreatePRComment(prInfo.Org, prInfo.Repo, prInfo.Number, comment)
	}

	return bot.cli.ClosePR(prInfo.Org, prInfo.Repo, prInfo.Number)
}

func (bot *robot) handleIssue(ne giteeclient.IssueNoteEvent, log *logrus.Entry) error {
	org, repo := ne.GetOrgRep()
	commenter := ne.GetCommenter()
	number := ne.GetIssueNumber()
	author := ne.GetIssueAuthor()

	if ne.IsIssueClosed() && reopenRe.MatchString(ne.GetComment()) {
		return bot.openIssue(org, repo, number, commenter, author)
	}

	if ne.IsIssueOpen() && closeRe.MatchString(ne.GetComment()) {
		return bot.closeIssue(org, repo, number, commenter, author)
	}

	return nil
}

func (bot *robot) openIssue(org, repo, number, commenter, author string) error {
	v, err := bot.hasPermission(org, repo, commenter, author)
	if err != nil {
		return err
	}
	if !v {
		return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(issueOptionFailureMessage, commenter, "reopen"))
	}

	return bot.cli.ReopenIssue(org, repo, number)
}

func (bot *robot) closeIssue(org, repo, number, commenter, author string) error {
	v, err := bot.hasPermission(org, repo, commenter, author)
	if err != nil {
		return err
	}
	if !v {
		return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(issueOptionFailureMessage, commenter, "close"))
	}

	return bot.cli.CloseIssue(org, repo, number)
}

func (bot *robot) hasPermission(org, repo, commenter, author string) (bool, error) {
	if commenter == author {
		return true, nil
	}

	return bot.cli.IsCollaborator(org, repo, commenter)
}
