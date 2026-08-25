package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunShell(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/decks":
			w.Write([]byte(`[{"id":"d1","slug":"jp-n3","name":"일본어 N3","cardCount":12}]`))
		case "/api/due-count":
			w.Write([]byte(`{"count":3}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	in := strings.NewReader("decks\n\ndue\nmenu\nnope\nexit\n")
	var out, errOut bytes.Buffer
	if err := runShell(context.Background(), options{server: srv.URL}, in, &out, &errOut); err != nil {
		t.Fatalf("runShell: %v", err)
	}

	for _, want := range []string{"jp-n3", "일본어 N3", "복습할 카드: 3장"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("출력에 %q가 없다:\n%s", want, out.String())
		}
	}
	if !strings.Contains(errOut.String(), "셸 안에서는") {
		t.Errorf("셸 안 menu 명령을 막지 않았다:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "nope") {
		t.Errorf("모르는 명령을 오류로 알리지 않았다:\n%s", errOut.String())
	}
}

// 셸 안에서 만드는 트리에는 shell·menu가 없어야 한다(모드 재진입 금지).
func TestNestedTreeHasNoModeCommands(t *testing.T) {
	for _, name := range []string{"shell", "menu"} {
		for _, c := range newRootCmd(options{nested: true}).Commands() {
			if c.Name() == name {
				t.Errorf("중첩 트리에 %s 명령이 있다", name)
			}
		}
	}
}

// 옵션 없이 실행하면 메뉴 모드로 가고, 오타 난 명령은 오류가 되어야 한다.
func TestRootRunsMenuButRejectsUnknownArgs(t *testing.T) {
	root := newRootCmd(options{server: "http://localhost:8080"})
	if root.RunE == nil {
		t.Fatal("루트에 기본 동작(메뉴 모드)이 없다")
	}
	if err := root.Args(root, []string{"deks"}); err == nil {
		t.Error("오타 난 명령이 오류가 되지 않는다")
	}
}
