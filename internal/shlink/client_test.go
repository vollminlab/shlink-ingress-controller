package shlink_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vollminlab/shlink-ingress-controller/internal/shlink"
)

func TestGetShortURL_Found(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "GET", r.Method)
        assert.Equal(t, "/rest/v3/short-urls/radarr", r.URL.Path)
        assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"shortCode": "radarr", "longUrl": "https://radarr.vollminlab.com"})
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    result, err := c.GetShortURL("radarr")
    require.NoError(t, err)
    require.NotNil(t, result)
    assert.Equal(t, "radarr", result.ShortCode)
    assert.Equal(t, "https://radarr.vollminlab.com", result.LongURL)
}

func TestGetShortURL_NotFound(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotFound)
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    result, err := c.GetShortURL("missing")
    require.NoError(t, err)
    assert.Nil(t, result)
}

func TestGetShortURL_ServerError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    _, err := c.GetShortURL("slug")
    assert.Error(t, err)
}

func TestCreateShortURL_Success(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "POST", r.Method)
        assert.Equal(t, "/rest/v3/short-urls", r.URL.Path)
        assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
        var body map[string]string
        json.NewDecoder(r.Body).Decode(&body)
        assert.Equal(t, "radarr", body["customSlug"])
        assert.Equal(t, "https://radarr.vollminlab.com", body["longUrl"])
        w.WriteHeader(http.StatusCreated)
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    err := c.CreateShortURL("radarr", "https://radarr.vollminlab.com")
    assert.NoError(t, err)
}

func TestCreateShortURL_AlreadyExists(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusConflict)
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    err := c.CreateShortURL("radarr", "https://radarr.vollminlab.com")
    assert.NoError(t, err) // 409 is not an error
}

func TestDeleteShortURL_Success(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "DELETE", r.Method)
        assert.Equal(t, "/rest/v3/short-urls/radarr", r.URL.Path)
        assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
        w.WriteHeader(http.StatusNoContent)
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    err := c.DeleteShortURL("radarr")
    assert.NoError(t, err)
}

func TestDeleteShortURL_Error(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer srv.Close()

    c := shlink.New(srv.URL+"/rest/v3", "test-api-key")
    err := c.DeleteShortURL("radarr")
    assert.Error(t, err)
}
