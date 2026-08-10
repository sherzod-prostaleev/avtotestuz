package content_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/server"
	"avtotest.uz/backend/internal/testdb"
)

func setup(t *testing.T) *httptest.Server {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h, _, _ := server.New(config.Config{Env: "test", MediaBaseURL: "http://media.test"},
		server.Deps{Queries: sqlc.New(pool)})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, ts *httptest.Server, path string, out any) int {
	t.Helper()
	resp, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
	return resp.StatusCode
}

func TestVariantsAndDetail(t *testing.T) {
	ts := setup(t)
	var list struct {
		Data []struct {
			Number        int `json:"number"`
			QuestionCount int `json:"question_count"`
		} `json:"data"`
	}
	if code := get(t, ts, "/api/v1/variants", &list); code != 200 {
		t.Fatalf("code=%d", code)
	}
	if len(list.Data) != 2 || list.Data[0].QuestionCount != 20 {
		t.Fatalf("list=%+v", list.Data)
	}

	var detail struct {
		Data struct {
			Number    int `json:"number"`
			Questions []struct {
				Text    string `json:"text"`
				Answers []struct {
					Text string `json:"text"`
				} `json:"answers"`
			} `json:"questions"`
		} `json:"data"`
		Meta struct {
			Locale   string `json:"locale"`
			Fallback bool   `json:"fallback"`
		} `json:"meta"`
	}
	if code := get(t, ts, "/api/v1/variants/1?locale=ru", &detail); code != 200 {
		t.Fatalf("code=%d", code)
	}
	if len(detail.Data.Questions) != 20 || len(detail.Data.Questions[0].Answers) != 4 {
		t.Fatalf("questions=%d", len(detail.Data.Questions))
	}
	if !strings.HasPrefix(detail.Data.Questions[0].Text, "[ОБРАЗЕЦ]") || detail.Meta.Fallback {
		t.Fatalf("ru text=%q fallback=%v", detail.Data.Questions[0].Text, detail.Meta.Fallback)
	}

	// anti-cheat: correctness must never appear in payload
	raw := struct {
		Data json.RawMessage `json:"data"`
	}{}
	_ = get(t, ts, "/api/v1/variants/1", &raw)
	s := string(raw.Data)
	if strings.Contains(s, "is_correct") || strings.Contains(s, "correct_answer") {
		t.Fatal("correctness leaked in variant payload")
	}

	if code := get(t, ts, "/api/v1/variants/99", nil); code != 404 {
		t.Fatalf("want 404, got %d", code)
	}
	if code := get(t, ts, "/api/v1/variants/1?locale=en", nil); code != 400 {
		t.Fatalf("want 400 invalid locale, got %d", code)
	}
}

func TestCategoriesFallback(t *testing.T) {
	ts := setup(t)
	var cats struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
		Meta struct {
			Fallback bool `json:"fallback"`
		} `json:"meta"`
	}
	if code := get(t, ts, "/api/v1/categories?locale=kaa", &cats); code != 200 {
		t.Fatalf("code=%d", code)
	}
	if len(cats.Data) != 4 || !cats.Meta.Fallback {
		t.Fatalf("kaa fallback expected: %+v meta=%+v", cats.Data, cats.Meta)
	}
}

func TestSignsAndQuestionDetail(t *testing.T) {
	ts := setup(t)
	var signs struct {
		Data []struct {
			Code          string `json:"code"`
			QuestionCount int    `json:"question_count"`
		} `json:"data"`
	}
	if code := get(t, ts, "/api/v1/signs?group=prohibiting", &signs); code != 200 {
		t.Fatalf("code=%d", code)
	}
	if len(signs.Data) != 2 {
		t.Fatalf("signs=%+v", signs.Data)
	}

	var sign struct {
		Data struct {
			QuestionIDs []string `json:"question_ids"`
		} `json:"data"`
	}
	if code := get(t, ts, "/api/v1/signs/3.27", &sign); code != 200 {
		t.Fatalf("code=%d", code)
	}
	if len(sign.Data.QuestionIDs) == 0 {
		t.Fatal("sign 3.27 must link questions (fixture links every 4th question)")
	}

	var qd struct {
		Data struct {
			ID       string `json:"id"`
			ImageURL string `json:"image_url"`
			Signs    []struct {
				Code string `json:"code"`
			} `json:"signs"`
			Explanation *struct {
				Blocks json.RawMessage `json:"blocks"`
			} `json:"explanation"`
		} `json:"data"`
	}
	if code := get(t, ts, "/api/v1/questions/"+sign.Data.QuestionIDs[0]+"?locale=uz-Latn", &qd); code != 200 {
		t.Fatalf("code=%d", code)
	}
	if len(qd.Data.Signs) != 2 {
		t.Fatalf("question signs=%+v", qd.Data.Signs)
	}
	if !strings.HasPrefix(qd.Data.ImageURL, "http://media.test/images/") {
		t.Fatalf("image url=%q", qd.Data.ImageURL)
	}
	if qd.Data.Explanation != nil {
		t.Fatal("public question detail must not expose answer-revealing explanation prose")
	}
}
