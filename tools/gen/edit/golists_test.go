package edit_test

import (
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const eventTypesSrc = `package entity

func EventTypes() []Event {
	return slices.Concat(
		taskEvents(),
		userEvents(), // the users
	)
}
`

func TestAppendArgAddsAnArgumentToAMultiLineCall(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, eventTypesSrc).AppendArg("EventTypes", "slices.Concat", "widgetEvents()")
	require.NoError(t, err)

	assert.Equal(t, `package entity

func EventTypes() []Event {
	return slices.Concat(
		taskEvents(),
		userEvents(), // the users
		widgetEvents(),
	)
}
`, applyGo(t, eventTypesSrc, ins))
}

func TestAppendArgAddsAnArgumentToAOneLineCall(t *testing.T) {
	t.Parallel()

	const src = "package p\n\nfunc f() {\n\tg(a, b)\n}\n"

	ins, err := parseGo(t, src).AppendArg("f", "g", "c")
	require.NoError(t, err)

	assert.Equal(t, "package p\n\nfunc f() {\n\tg(a, b, c)\n}\n", applyGo(t, src, ins))
}

func TestAppendArgNeedsExactlyOneCall(t *testing.T) {
	t.Parallel()

	g := parseGo(t, "package p\n\nfunc f() {\n\tg(a)\n\tg(b)\n}\n\nfunc h() {}\n")

	for name, args := range map[string][2]string{
		"no such func": {"missing", "g"},
		"no such call": {"h", "g"},
		"two calls":    {"f", "g"},
	} {
		_, err := g.AppendArg(args[0], args[1], "c")
		require.ErrorIs(t, err, edit.ErrAnchor, name)
	}
}

const subscribersSrc = `package app

func subscribers(cfg *config.Config, pg *postgres.Postgres) []repo.Subscriber {
	return []repo.Subscriber{
		persistTaskRepo.NewProjector(pg),
		persistUserRepo.NewProjector(pg),
		userevent.SendWelcomeEmail{Notifier: notify.New(queue.New(pg), loginURL(cfg))},
	}
}
`

func TestAfterLastElementGoesAfterTheLastMatch(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, subscribersSrc).AfterLastElement("subscribers", edit.Call("*.NewProjector"),
		"persistWidgetRepo.NewProjector(pg)")
	require.NoError(t, err)

	assert.Equal(t, `package app

func subscribers(cfg *config.Config, pg *postgres.Postgres) []repo.Subscriber {
	return []repo.Subscriber{
		persistTaskRepo.NewProjector(pg),
		persistUserRepo.NewProjector(pg),
		persistWidgetRepo.NewProjector(pg),
		userevent.SendWelcomeEmail{Notifier: notify.New(queue.New(pg), loginURL(cfg))},
	}
}
`, applyGo(t, subscribersSrc, ins))
}

const mapSrc = `package gqlerr

func Map(err error) error {
	for _, m := range []mapping{
		{entity.ErrTaskNotFound, _codeNotFound, "task not found"},
		// Any message can produce these, whatever its domain.
		{entity.ErrReadModelBehind, _codeUnavailable, "saved, but not yet readable"},
		{entity.ErrNotImplemented, _codeNotImplemented, "not implemented"},
	} {
		_ = m
	}

	return nil
}
`

func TestBeforeFirstElementGoesAboveTheMatchAndItsComment(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, mapSrc).BeforeFirstElement("Map", edit.Leading("entity.ErrReadModelBehind"),
		"{entity.ErrWidgetNotFound, _codeNotFound, \"widget not found\"}")
	require.NoError(t, err)

	assert.Equal(t, `package gqlerr

func Map(err error) error {
	for _, m := range []mapping{
		{entity.ErrTaskNotFound, _codeNotFound, "task not found"},
		{entity.ErrWidgetNotFound, _codeNotFound, "widget not found"},
		// Any message can produce these, whatever its domain.
		{entity.ErrReadModelBehind, _codeUnavailable, "saved, but not yet readable"},
		{entity.ErrNotImplemented, _codeNotImplemented, "not implemented"},
	} {
		_ = m
	}

	return nil
}
`, applyGo(t, mapSrc, ins))
}

func TestElementsNeedOneMatchingLiteral(t *testing.T) {
	t.Parallel()

	g := parseGo(t, subscribersSrc)

	_, err := g.AfterLastElement("subscribers", edit.Call("*.NewGuard"), "x")
	require.ErrorIs(t, err, edit.ErrAnchor, "no match")

	_, err = parseGo(t, mapSrc).BeforeFirstElement("Map", edit.Leading("entity.ErrMissing"), "x")
	require.ErrorIs(t, err, edit.ErrAnchor, "no leading match")

	twoLiterals := "package p\n\nfunc f() {\n\ta := []T{\n\t\tx.New(),\n\t}\n\tb := []T{\n\t\ty.New(),\n\t}\n}\n"
	_, err = parseGo(t, twoLiterals).AfterLastElement("f", edit.Call("*.New"), "z.New()")
	require.ErrorIs(t, err, edit.ErrAnchor, "matches in two literals")
}

const registerSrc = `package app

func registerHandlers(cmds *cqrs.Commands, qs *cqrs.Queries) error {
	signIn, err := newSSO(cfg)
	if err != nil {
		return err
	}

	task.Register(cmds, qs, task.Deps{Store: store})
	exchange.Register(cmds, qs, exchange.Deps{
		Store: store,
	})
	mail.Register(cmds, mailDeps)

	return nil
}
`

func TestAfterLastStmtGoesAfterTheLastMatchingStatement(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, registerSrc).AfterLastStmt("registerHandlers", edit.Call("*.Register"),
		"widget.Register(cmds, qs, widget.Deps{\n\tStore: store,\n})")
	require.NoError(t, err)

	assert.Equal(t, `package app

func registerHandlers(cmds *cqrs.Commands, qs *cqrs.Queries) error {
	signIn, err := newSSO(cfg)
	if err != nil {
		return err
	}

	task.Register(cmds, qs, task.Deps{Store: store})
	exchange.Register(cmds, qs, exchange.Deps{
		Store: store,
	})
	mail.Register(cmds, mailDeps)
	widget.Register(cmds, qs, widget.Deps{
		Store: store,
	})

	return nil
}
`, applyGo(t, registerSrc, ins))
}

func TestAfterLastStmtMatchesAnAssignedCall(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, registerSrc).AfterLastStmt("registerHandlers", edit.Call("newSSO"), "other := 1")
	require.NoError(t, err)
	assert.Contains(t, applyGo(t, registerSrc, ins), "signIn, err := newSSO(cfg)\n\tother := 1\n")

	_, err = parseGo(t, registerSrc).AfterLastStmt("registerHandlers", edit.Call("*.Deregister"), "x")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

// The matching statements are the place only when they are one run: split in two, the new one
// could go after either, so the place is found twice.
func TestAfterLastStmtRefusesMatchesInTwoRuns(t *testing.T) {
	t.Parallel()

	split := strings.Replace(registerSrc, "\tmail.Register(cmds, mailDeps)\n", "\tlog()\n\tmail.Register(cmds, mailDeps)\n", 1)

	_, err := parseGo(t, split).AfterLastStmt("registerHandlers", edit.Call("*.Register"), "x")

	require.ErrorIs(t, err, edit.ErrAnchor)
	assert.Contains(t, err.Error(), "found a run of statements in registerHandlers that are each a call to *.Register 2 times")
}

const resolverSrc = `package graphql

type query struct {
	taskQuery
	exchangeQuery
}

func newResolver(b cqrs.Buses) *resolver {
	taskQ, taskM := task.New(b)

	return &resolver{
		query:    query{taskQuery: taskQ, exchangeQuery: exchange.New(b)},
		mutation: mutation{taskMutation: taskM},
	}
}
`

func TestAppendFieldAddsToANamedStruct(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, resolverSrc).AppendField("query", "widgetQuery")
	require.NoError(t, err)
	assert.Contains(t, applyGo(t, resolverSrc, ins), "type query struct {\n\ttaskQuery\n\texchangeQuery\n\twidgetQuery\n}\n")

	_, err = parseGo(t, resolverSrc).AppendField("mutation", "widgetMutation")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

func TestAppendElementAddsAKeyToOneLiteral(t *testing.T) {
	t.Parallel()

	g := parseGo(t, resolverSrc)

	q, err := g.AppendElement("newResolver", "query", "widgetQuery: widgetQ")
	require.NoError(t, err)

	m, err := g.AppendElement("newResolver", "mutation", "widgetMutation: widgetM")
	require.NoError(t, err)

	got := applyGo(t, resolverSrc, q, m)
	assert.Contains(t, got, "query{taskQuery: taskQ, exchangeQuery: exchange.New(b), widgetQuery: widgetQ},")
	assert.Contains(t, got, "mutation{taskMutation: taskM, widgetMutation: widgetM},")

	_, err = g.AppendElement("newResolver", "subscription", "x: y")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

func TestAtEndAppendsToTheFile(t *testing.T) {
	t.Parallel()

	const src = "package p\n\nfunc a() {}\n"

	got := applyGo(t, src, parseGo(t, src).AtEnd("// b -.\nfunc b() {}"))

	assert.Equal(t, "package p\n\nfunc a() {}\n\n// b -.\nfunc b() {}\n", got)
}

func TestBeforeFirstElementIgnoresATrailingCommentOnTheLineAbove(t *testing.T) {
	t.Parallel()

	const src = "package p\n\nfunc f() {\n\t_ = []T{\n\t\t{a, 1}, // a's\n\t\t{b, 2},\n\t}\n}\n"

	ins, err := parseGo(t, src).BeforeFirstElement("f", edit.Leading("b"), "{c, 3}")
	require.NoError(t, err)

	assert.Equal(t, "package p\n\nfunc f() {\n\t_ = []T{\n\t\t{a, 1}, // a's\n\t\t{c, 3},\n\t\t{b, 2},\n\t}\n}\n",
		applyGo(t, src, ins))
}
