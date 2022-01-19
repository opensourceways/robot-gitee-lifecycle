package main

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/opensourceways/community-robot-lib/config"
	framework "github.com/opensourceways/community-robot-lib/robot-gitee-framework"
	sdk "github.com/opensourceways/go-gitee/gitee"
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

func (bot *robot) NewConfig() config.Config {
	return &configuration{}
}

func (bot *robot) getConfig(cfg config.Config) (*configuration, error) {
	if c, ok := cfg.(*configuration); ok {
		return c, nil
	}
	return nil, errors.New("can't convert to configuration")
}

func (bot *robot) RegisterEventHandler(p framework.HandlerRegitster) {
	p.RegisterNoteEventHandler(bot.handleNoteEvent)
}

func (bot *robot) handleNoteEvent(e *sdk.NoteEvent, cfg config.Config, log *logrus.Entry) error {
	if !e.IsCreatingCommentEvent() {
		log.Debug("Event is not a creation of a comment for PR or issue, skipping.")
		return nil
	}

	config, err := bot.getConfig(cfg)
	if err != nil {
		return err
	}

	org, repo := e.GetOrgRepo()
	if config.configFor(org, repo) == nil {
		log.Debug("ignore this event, because of no configuration for this repo.")
		return nil
	}

	if e.IsPullRequest() {
		return bot.handlePullRequest(e, log)
	}

	if e.IsIssue() {
		return bot.handleIssue(e, log)
	}

	return nil
}

func (bot *robot) handlePullRequest(e *sdk.NoteEvent, log *logrus.Entry) error {
	if !e.IsPROpen() || !closeRe.MatchString(e.GetComment().GetBody()) {
		return nil
	}

	org, repo := e.GetOrgRepo()
	commenter := e.GetCommenter()

	v, err := bot.hasPermission(org, repo, commenter, e.GetPRAuthor())
	if err != nil {
		return err
	}

	number := e.GetPRNumber()
	if !v {
		comment := fmt.Sprintf(prOptionFailureMessage, commenter, "close")
		return bot.cli.CreatePRComment(org, repo, number, comment)
	}

	return bot.cli.ClosePR(org, repo, number)
}

func (bot *robot) handleIssue(e *sdk.NoteEvent, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()
	commenter := e.GetCommenter()
	number := e.GetIssueNumber()
	author := e.GetIssueAuthor()
	comment := e.GetComment().GetBody()

	if e.IsIssueClosed() && reopenRe.MatchString(comment) {
		return bot.openIssue(org, repo, number, commenter, author)
	}

	if e.IsIssueOpen() && closeRe.MatchString(comment) {
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
