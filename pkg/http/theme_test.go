package http

import (
	"io/ioutil"
	"path/filepath"

	"os"
	"strings"
	"testing"
)

func touchFile(path, filename string) error {
	target := filepath.Join(path, filename)
	return ioutil.WriteFile(target, []byte{}, 0644)
}

func TestThemeFiles(t *testing.T) {
	themePath, err := ioutil.TempDir("", "alice-lg-tmp-theme")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(themePath)

	// Create some "stylesheets" and a "script"
	touchFile(themePath, "style.css")
	touchFile(themePath, "extra.css")
	touchFile(themePath, "script.js")

	// Load theme
	theme := NewTheme(ThemeConfig{
		BasePath: "/theme",
		Path:     themePath,
	})

	if err != nil {
		t.Error(err)
	}

	// Check file presence
	scripts := theme.Scripts()
	if len(scripts) != 1 {
		t.Error("Expected one script file: script.js")
	}

	stylesheets := theme.Stylesheets()
	if len(stylesheets) != 2 {
		t.Error("Expected two stylesheets: {style, extra}.css")
	}

	// Check uri / path mapping
	script := scripts[0]
	if script != "script.js" {
		t.Error("Expected script.js to be included in scripts")
	}
}

func TestThemeIncludeHash(t *testing.T) {
	themePath, err := ioutil.TempDir("", "alice-lg-tmp-theme")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(themePath)

	// Create some "stylesheets" and a "script"
	touchFile(themePath, "style.css")

	theme := NewTheme(ThemeConfig{
		BasePath: "/theme",
		Path:     themePath,
	})

	hash := theme.HashInclude("style.css")
	if hash == "" {
		t.Error("Something went wrong with hashing")
	}

	t.Log("Filehash:", hash)

}

func TestThemeIncludes(t *testing.T) {
	themePath, err := ioutil.TempDir("", "alice-lg-tmp-theme")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(themePath)

	// Create some "stylesheets" and a "script"
	touchFile(themePath, "style.css")
	touchFile(themePath, "extra.css")
	touchFile(themePath, "script.js")

	// Load theme
	theme := NewTheme(ThemeConfig{
		BasePath: "/theme",
		Path:     themePath,
	})

	stylesHtml := theme.StylesheetIncludes()
	scriptsHtml := theme.ScriptIncludes()

	if !strings.HasPrefix(scriptsHtml, "<script") {
		t.Error("Script include should start with <script")
	}
	if strings.Index(scriptsHtml, "script.js") == -1 {
		t.Error("Scripts include should contain script.js")
	}

	if !strings.HasPrefix(stylesHtml, "<link") {
		t.Error("Stylesheet include should start with <link")
	}
	if strings.Index(stylesHtml, "extra.css") == -1 {
		t.Error("Stylesheet include should contain extra.css")
	}
	if strings.Index(stylesHtml, "script.js") != -1 {
		t.Error("Stylesheet include should not contain script.js")
	}

}
