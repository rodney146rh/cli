package config

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func Test_parseConfig(t *testing.T) {
	defer stubConfig(`---
hosts:
  github.com:
    user: monalisa
    oauth_token: OTOKEN
`, "")()
	config, err := parseConfig("config.yml")
	assert.NoError(t, err)
	user, err := config.Get("github.com", "user")
	assert.NoError(t, err)
	assert.Equal(t, "monalisa", user)
	token, err := config.Get("github.com", "oauth_token")
	assert.NoError(t, err)
	assert.Equal(t, "OTOKEN", token)
}

func Test_parseConfig_multipleHosts(t *testing.T) {
	defer stubConfig(`---
hosts:
  example.com:
    user: wronguser
    oauth_token: NOTTHIS
  github.com:
    user: monalisa
    oauth_token: OTOKEN
`, "")()
	config, err := parseConfig("config.yml")
	assert.NoError(t, err)
	user, err := config.Get("github.com", "user")
	assert.NoError(t, err)
	assert.Equal(t, "monalisa", user)
	token, err := config.Get("github.com", "oauth_token")
	assert.NoError(t, err)
	assert.Equal(t, "OTOKEN", token)
}

func Test_parseConfig_hostsFile(t *testing.T) {
	defer stubConfig("", `---
github.com:
  user: monalisa
  oauth_token: OTOKEN
`)()
	config, err := parseConfig("config.yml")
	assert.NoError(t, err)
	user, err := config.Get("github.com", "user")
	assert.NoError(t, err)
	assert.Equal(t, "monalisa", user)
	token, err := config.Get("github.com", "oauth_token")
	assert.NoError(t, err)
	assert.Equal(t, "OTOKEN", token)
}

func Test_parseConfig_hostFallback(t *testing.T) {
	defer stubConfig(`---
git_protocol: ssh
`, `---
github.com:
    user: monalisa
    oauth_token: OTOKEN
example.com:
    user: wronguser
    oauth_token: NOTTHIS
    git_protocol: https
`)()
	config, err := parseConfig("config.yml")
	assert.NoError(t, err)
	val, err := config.Get("example.com", "git_protocol")
	assert.NoError(t, err)
	assert.Equal(t, "https", val)
	val, err = config.Get("github.com", "git_protocol")
	assert.NoError(t, err)
	assert.Equal(t, "ssh", val)
	val, err = config.Get("nonexistent.io", "git_protocol")
	assert.NoError(t, err)
	assert.Equal(t, "ssh", val)
}

func Test_parseConfig_migrateConfig(t *testing.T) {
	defer stubConfig(`---
github.com:
  - user: keiyuri
    oauth_token: 123456
`, "")()

	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()
	defer StubBackupConfig()()

	_, err := parseConfig("config.yml")
	assert.NoError(t, err)

	expectedHosts := `github.com:
    user: keiyuri
    oauth_token: "123456"
`

	assert.Equal(t, expectedHosts, hostsBuf.String())
	assert.NotContains(t, mainBuf.String(), "github.com")
	assert.NotContains(t, mainBuf.String(), "oauth_token")
}

func Test_parseConfigFile(t *testing.T) {
	tests := []struct {
		contents string
		wantsErr bool
	}{
		{
			contents: "",
			wantsErr: true,
		},
		{
			contents: " ",
			wantsErr: false,
		},
		{
			contents: "\n",
			wantsErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("contents: %q", tt.contents), func(t *testing.T) {
			defer stubConfig(tt.contents, "")()
			_, yamlRoot, err := parseConfigFile("config.yml")
			if tt.wantsErr != (err != nil) {
				t.Fatalf("got error: %v", err)
			}
			if tt.wantsErr {
				return
			}
			assert.Equal(t, yaml.MappingNode, yamlRoot.Content[0].Kind)
			assert.Equal(t, 0, len(yamlRoot.Content[0].Content))
		})
	}
}

func Test_ConfigDir(t *testing.T) {
	tests := []struct {
		name        string
		onlyWindows bool
		env         map[string]string
		output      string
	}{
		{
			name:   "no envVars",
			output: ".config.gh",
			env: map[string]string{
				"AppData": "",
			},
		},
		{
			name: "GH_CONFIG_DIR specified",
			env: map[string]string{
				"GH_CONFIG_DIR": "/tmp/gh_config_dir",
			},
			output: ".tmp.gh_config_dir",
		},
		{
			name: "XDG_CONFIG_HOME specified",
			env: map[string]string{
				"XDG_CONFIG_HOME": "/tmp",
			},
			output: ".tmp.gh",
		},
		{
			name: "GH_CONFIG_DIR and XDG_CONFIG_HOME specified",
			env: map[string]string{
				"GH_CONFIG_DIR":   "/tmp/gh_config_dir",
				"XDG_CONFIG_HOME": "/tmp",
			},
			output: ".tmp.gh_config_dir",
		},
		{
			name:        "AppData specified",
			onlyWindows: true,
			env: map[string]string{
				"AppData": "/tmp/",
			},
			output: ".tmp.GitHub CLI",
		},
		{
			name:        "GH_CONFIG_DIR and AppData specified",
			onlyWindows: true,
			env: map[string]string{
				"GH_CONFIG_DIR": "/tmp/gh_config_dir",
				"AppData":       "/tmp",
			},
			output: ".tmp.gh_config_dir",
		},
		{
			name:        "XDG_CONFIG_HOME and AppData specified",
			onlyWindows: true,
			env: map[string]string{
				"XDG_CONFIG_HOME": "/tmp",
				"AppData":         "/tmp",
			},
			output: ".tmp.gh",
		},
	}

	for _, tt := range tests {
		if tt.onlyWindows && runtime.GOOS != "windows" {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != nil {
				for k, v := range tt.env {
					old := os.Getenv(k)
					os.Setenv(k, v)
					defer os.Setenv(k, old)
				}
			}

			defer stubMigrateConfigDir()()
			assert.Regexp(t, tt.output, ConfigDir())
		})
	}
}
