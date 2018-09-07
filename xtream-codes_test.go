package xtreamcodes

// There's a 99% chance that this is a terrible way to write tests but I just needed a quick way to validate the JSON schemas.
// Apologies in advance!

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onsi/gomega"
)

var expectedContents = make(map[string]map[string][]byte)
var types = []string{"live", "vod", "series"}
var tsURL string

func walkFunc(path string, f os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if f.IsDir() || f.Name() == "testData" {
		return nil
	}
	bytes, err := ioutil.ReadFile(path) // path is the path to the file.
	if err != nil {
		return err
	}
	splitPath := strings.Split(path, "/")
	if expectedContents[f.Name()] == nil {
		expectedContents[f.Name()] = make(map[string][]byte)
	}
	expectedContents[f.Name()][splitPath[1]] = bytes
	return nil
}

func TestMain(m *testing.M) {
	if err := filepath.Walk("testData", walkFunc); err != nil {
		panic(err)
	}

	serv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		qs := r.URL.Query()
		provider := strings.ToLower(strings.Replace(qs.Get("username"), "_USERNAME", "", -1))
		action := fmt.Sprintf("%s.json", qs.Get("action"))

		if action == ".json" {
			action = "auth.json"
		}

		// fmt.Printf("httptest: Serving request for provider %s action %s\n", provider, action)

		w.Write(expectedContents[action][provider])
	}))
	defer serv.Close()

	tsURL = serv.URL

	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	for providerName, expectedContent := range expectedContents["auth.json"] {
		t.Run(providerName, func(t *testing.T) {
			t.Parallel()
			xc := getClient(t, providerName)
			t.Logf("%s client successfully created!", providerName)

			marshalled, marshalErr := json.Marshal(AuthenticationResponse{
				UserInfo:   xc.UserInfo,
				ServerInfo: xc.ServerInfo,
			})
			if marshalErr != nil {
				t.Errorf("error marshalling auth response back to json: %s", marshalErr)
				t.Fail()
				return
			}

			g := gomega.NewGomegaWithT(t)

			g.Expect(string(expectedContent)).To(gomega.MatchJSON(string(marshalled)))
		})
	}
}

func TestGetCategories(t *testing.T) {
	for _, xType := range types {
		t.Run(xType, func(t *testing.T) {
			t.Parallel()
			filePath := fmt.Sprintf("get_%s_categories.json", xType)
			for providerName, expectedContent := range expectedContents[filePath] {
				t.Run(providerName, func(t *testing.T) {
					t.Parallel()
					xc := getClient(t, providerName)

					cats, catsErr := xc.GetCategories(xType)
					if catsErr != nil {
						t.Errorf("error getting %s categories: %s", xType, catsErr)
						t.Fail()
						return
					}

					marshalled, marshalErr := json.Marshal(cats)
					if marshalErr != nil {
						t.Errorf("error marshalling %s response back to json: %s", xType, marshalErr)
						t.Fail()
						return
					}

					g := gomega.NewGomegaWithT(t)

					g.Expect(string(expectedContent)).To(gomega.MatchJSON(string(marshalled)))

					t.Logf("Got %s categories from %s successfully!", xType, providerName)
				})
			}
		})
	}
}

func TestGetStreams(t *testing.T) {
	for _, xType := range types {
		t.Run(xType, func(t *testing.T) {
			if xType == "series" {
				return
			}
			t.Parallel()
			filePath := fmt.Sprintf("get_%s_streams.json", xType)
			for providerName, expectedContent := range expectedContents[filePath] {
				t.Run(providerName, func(t *testing.T) {
					t.Parallel()
					xc := getClient(t, providerName)

					streams, streamsErr := xc.GetStreams(xType, "")
					if streamsErr != nil {
						t.Errorf("error getting %s streams: %s", xType, streamsErr)
						t.Fail()
						return
					}

					t.Logf("Calling json.Marshal for %s streams from %s", xType, providerName)

					marshalled, marshalErr := json.Marshal(streams)
					if marshalErr != nil {
						t.Errorf("error marshalling %s response back to json: %s", xType, marshalErr)
						t.Fail()
						return
					}

					t.Logf("Calling assert.JSONEq for %s streams from %s", xType, providerName)

					g := gomega.NewGomegaWithT(t)

					g.Expect(string(expectedContent)).To(gomega.MatchJSON(string(marshalled)))

					t.Logf("Got %s streams from %s successfully!", xType, providerName)
				})
			}
		})
	}
}

// func TestGetSeries(t *testing.T) {
// 	for providerName, expectedContent := range expectedContents["get_series.json"] {
// 		t.Run(providerName, func(t *testing.T) {
// 			t.Parallel()
// 			xc := getClient(t, providerName)

// 			streams, streamsErr := xc.GetSeries("")
// 			if streamsErr != nil {
// 				t.Errorf("error getting series: %s", streamsErr)
// 				t.Fail()
// 				return
// 			}

// 			t.Logf("Calling json.Marshal for series from %s", providerName)

// 			marshalled, marshalErr := json.Marshal(streams)
// 			if marshalErr != nil {
// 				t.Errorf("error marshalling series response back to json: %s", marshalErr)
// 				t.Fail()
// 				return
// 			}

// 			t.Logf("Calling assert.JSONEq for series from %s", providerName)

// 			t.Logf("%s\n\n\n%s", string(expectedContent), string(marshalled))

// 			g := gomega.NewGomegaWithT(t)

// 			g.Expect(string(expectedContent)).To(gomega.MatchJSON(string(marshalled)))

// 			t.Logf("Got series from %s successfully!", providerName)
// 		})
// 	}
// }

func getClient(t *testing.T, providerName string) *XtreamClient {
	t.Helper()
	xc, xcErr := NewClient(fmt.Sprintf("%s_USERNAME", strings.ToUpper(providerName)), fmt.Sprintf("%s_PASSWORD", strings.ToUpper(providerName)), tsURL)
	if xcErr != nil {
		t.Errorf("NewClient() returned an error: %s", xcErr)
		t.Fail()
		return nil
	}
	return xc
}
