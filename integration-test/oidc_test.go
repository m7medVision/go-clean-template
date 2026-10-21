package integration_test

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// kcLoginForm finds the action of Keycloak's username/password form.
var kcLoginForm = regexp.MustCompile(`(?s)id="kc-form-login"[^>]*action="([^"]+)"`)

type oidcBegin struct {
	URL           string `json:"url"`
	CeremonyToken string `json:"ceremony_token"`
}

// ssoSignIn signs in through Keycloak as one of the realm's dev users (keycloak/realm.json, the
// password is the username) the way a browser does: begin, the provider's login form, and finish
// with the code and state from the redirect back to the web client, which is not followed.
func ssoSignIn(t *testing.T, username string) string {
	t.Helper()

	var begin oidcBegin

	postJSON(t, basePathV1+"/auth/oidc/begin", "", nil, http.StatusOK, &begin)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}

	browser := &http.Client{
		Jar:           jar,
		Timeout:       requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	page := browse(t, browser, http.MethodGet, begin.URL, nil)

	match := kcLoginForm.FindStringSubmatch(page)
	if match == nil {
		t.Fatalf("ssoSignIn: no Keycloak login form at %s", begin.URL)
	}

	form := url.Values{"username": {username}, "password": {username}}

	location := redirectOf(t, browser, html.UnescapeString(match[1]), form)
	callback := location.Query()

	if location.Path != "/sso/callback" || callback.Get("code") == "" {
		t.Fatalf("ssoSignIn: expected a redirect to /sso/callback with a code, got %s", location)
	}

	var token struct {
		Token string `json:"token"`
	}

	postJSON(t, basePathV1+"/auth/oidc/finish", begin.CeremonyToken,
		map[string]string{"code": callback.Get("code"), "state": callback.Get("state")}, http.StatusOK, &token)

	return token.Token
}

func postJSON(t *testing.T, target, ceremony string, body any, want int, out any) {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if ceremony != "" {
		req.Header.Set("X-Ceremony-Token", ceremony)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != want {
		t.Fatalf("POST %s: expected %d, got %d", target, want, resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("POST %s: decode: %v", target, err)
		}
	}
}

func browse(t *testing.T, client *http.Client, method, target string, form url.Values) string {
	t.Helper()

	resp := browserRequest(t, client, method, target, form)
	defer resp.Body.Close()

	page, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read: %v", method, target, err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: expected 200, got %d", method, target, resp.StatusCode)
	}

	return string(page)
}

func redirectOf(t *testing.T, client *http.Client, target string, form url.Values) *url.URL {
	t.Helper()

	resp := browserRequest(t, client, http.MethodPost, target, form)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST %s: expected a redirect, got %d (wrong password, or a required action?)", target, resp.StatusCode)
	}

	location, err := resp.Location()
	if err != nil {
		t.Fatalf("POST %s: Location: %v", target, err)
	}

	return location
}

func browserRequest(t *testing.T, client *http.Client, method, target string, form url.Values) *http.Response {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := client.Do(req)
	if err != nil && !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("%s %s: %v", method, target, err)
	}

	return resp
}

func profileID(t *testing.T, token string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/user/profile", http.NoBody, token)
	if err != nil {
		t.Fatalf("GET /v1/user/profile: %v", err)
	}

	defer resp.Body.Close()

	return parseJSON[struct {
		ID string `json:"id"`
	}](t, resp).ID
}

func TestSSOStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doWebRequestWithTimeout(ctx, http.MethodGet, basePathV1+"/auth/oidc", http.NoBody)
	if err != nil {
		t.Fatalf("GET /v1/auth/oidc: %v", err)
	}

	defer resp.Body.Close()

	if !parseJSON[struct {
		Enabled bool `json:"enabled"`
	}](t, resp).Enabled {
		t.Fatal("GET /v1/auth/oidc: expected SSO to be enabled against the compose Keycloak")
	}
}

func TestSSOSignInReusesTheUser(t *testing.T) {
	first := profileID(t, ssoSignIn(t, "alice"))
	second := profileID(t, ssoSignIn(t, "alice"))

	if first == "" || first != second {
		t.Fatalf("expected the second SSO sign-in to reuse user %q, got %q", first, second)
	}
}

// The admin side of TestAdminTasksRefuseLocalUser: alice holds app-admin in Keycloak, which
// OIDC_ROLE_MAP maps to admin, so she lists bob's task on every transport; bob is refused.
func TestSSOAdminListsEveryUsersTasks(t *testing.T) {
	bob := ssoSignIn(t, "bob")
	bobTask := httpCreateTask(t, bob, "bob's task for the admin list", "")
	alice := ssoSignIn(t, "alice")

	t.Run("REST", func(t *testing.T) {
		tasks := adminTasksREST(t, alice, http.StatusOK)

		if !slices.ContainsFunc(tasks, func(task taskResponse) bool { return task.ID == bobTask.ID }) {
			t.Fatalf("GET /v1/admin/tasks: bob's task %s is not in alice's list", bobTask.ID)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		if resp := graphqlDo(t, alice, `query { allTasks(limit: 1) { total } }`, nil); len(resp.Errors) != 0 {
			t.Fatalf("allTasks as admin: %+v", resp.Errors)
		}
	})

	t.Run("gRPC", func(t *testing.T) {
		if total := adminTasksGRPC(t, alice); total < 1 {
			t.Fatalf("ListAllTasks as admin: expected bob's task in the total, got %d", total)
		}
	})

	t.Run("a user is refused", func(t *testing.T) {
		adminTasksREST(t, bob, http.StatusForbidden)
	})
}

// adminTasksREST lists every user's tasks as token, expecting status want.
func adminTasksREST(t *testing.T, token string, want int) []taskResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/admin/tasks?limit=100", http.NoBody, token)
	if err != nil {
		t.Fatalf("GET /v1/admin/tasks: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != want {
		t.Fatalf("GET /v1/admin/tasks: expected %d, got %d", want, resp.StatusCode)
	}

	if want != http.StatusOK {
		return nil
	}

	return parseJSON[struct {
		Tasks []taskResponse `json:"tasks"`
	}](t, resp).Tasks
}

func adminTasksGRPC(t *testing.T, token string) int32 {
	t.Helper()

	conn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	defer conn.Close()

	resp, err := protov1.NewTaskServiceClient(conn).ListAllTasks(grpcAuthCtx(t, token), &protov1.ListTasksRequest{Limit: 1})
	if err != nil {
		t.Fatalf("ListAllTasks as admin: %v", err)
	}

	return resp.GetTotal()
}
