package alerting

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tests/testinfra"
	"github.com/stretchr/testify/require"
)

func TestAlertmanagerConfigurationIsTransactional(t *testing.T) {
	dir, path := testinfra.CreateGrafDir(t, testinfra.GrafanaOpts{
		EnableFeatureToggles: []string{"ngalert"},
		AnonymousUserRole:    models.ROLE_EDITOR,
	})

	store := testinfra.SetUpDatabase(t, dir)
	grafanaListedAddr := testinfra.StartGrafana(t, dir, path, store)
	alertConfigURL := fmt.Sprintf("http://%s/api/alertmanager/grafana/config/api/v1/alerts", grafanaListedAddr)

	// On a blank start with no configuration, it saves and delivers the default configuration.
	{
		resp := getRequest(t, alertConfigURL, http.StatusOK) // nolint
		require.JSONEq(t, defaultAlertmanagerConfigJSON, getBody(t, resp.Body))
	}

	// When creating new configuration, if it fails to apply - it does not save it.
	{
		payload := `
{
  "template_files": {},
  "alertmanager_config": {
    "route": {
      "receiver": "webhook_test",
      "group_by": [
        "alertname"
      ]
    },
    "templates": [],
    "receivers": [
      {
        "name": "Kyle Email",
        "grafana_managed_receiver_configs": [
          {
            "uid": "",
            "name": "Kyle Email",
            "type": "email",
            "sendReminder": false,
            "disableResolveMessage": true,
            "frequency": "",
            "isDefault": false,
            "settings": {
              "addresses": "kyle@grafana.com",
              "singleEmail": false
            },
            "secureSettings": null,
            "Result": null
          }
        ]
      },
      {
        "name": "webhook_test",
        "grafana_managed_receiver_configs": [
          {
            "uid": "",
            "name": "webhook_test",
            "type": "slack",
            "sendReminder": false,
            "disableResolveMessage": false,
            "frequency": "",
            "isDefault": false,
            "settings": {
              "recipient": "#unified-alerting-test",
              "text": "  Kyle\n  {{ range .Alerts }} {{ .Labels.alertname }}: {{ .Annotations.description }}\n{{.Labels.__value__}}\n     {{ range $k, $v := .Labels }}{{ $k }}:{{ $v }}\n     {{ end }}\n  {{ end }}",
              "username": "kyle"
            },
            "secureSettings": {
              "token": "UEN2bWdqOWwXI13bvCw7gqhh8OwiwfNb",
              "url": "Yll6ampqQ0sgxEEraCJHKV3jGDH1j8RV8iUU+iDd25olVe1x3lwBqNeFEu2Wkw=="
            },
            "Result": null
          }
        ]
      }
    ]
  }
}
`
		resp := postRequest(t, alertConfigURL, payload, http.StatusBadRequest) // nolint
		require.JSONEq(t, "{\"error\":\"alert validation error: token must be specified when using the Slack chat API\", \"message\":\"failed to save and apply Alertmanager configuration\"}", getBody(t, resp.Body))

		resp = getRequest(t, alertConfigURL, http.StatusOK) // nolint
		require.JSONEq(t, defaultAlertmanagerConfigJSON, getBody(t, resp.Body))
	}
}

func TestAlertmanagerConfigurationPersistSecrets(t *testing.T) {
	dir, path := testinfra.CreateGrafDir(t, testinfra.GrafanaOpts{
		EnableFeatureToggles: []string{"ngalert"},
		AnonymousUserRole:    models.ROLE_EDITOR,
	})

	store := testinfra.SetUpDatabase(t, dir)
	grafanaListedAddr := testinfra.StartGrafana(t, dir, path, store)
	alertConfigURL := fmt.Sprintf("http://%s/api/alertmanager/grafana/config/api/v1/alerts", grafanaListedAddr)

	// create a new configuration that has a secret
	{
		payload := `
{
	"template_files": {},
	"alertmanager_config": {
		"route": {
			"receiver": "slack.receiver"
		},
		"templates": null,
		"receivers": [{
			"name": "slack.receiver",
			"grafana_managed_receiver_configs": [{
				"settings": {
					"recipient": "#unified-alerting-test"
				},
				"secureSettings": {
					"url": "http://averysecureurl.com/webhook"
				},
				"type": "slack",
				"sendReminder": true,
				"name": "slack.receiver",
				"disableResolveMessage": false
			}]
		}]
	}
}
`
		resp := postRequest(t, alertConfigURL, payload, http.StatusAccepted) // nolint
		require.JSONEq(t, `{"message":"configuration created"}`, getBody(t, resp.Body))
	}
	// Then, update the recipient
	{
		payload := `
{
	"template_files": {},
	"alertmanager_config": {
		"route": {
			"receiver": "slack.receiver"
		},
		"templates": null,
		"receivers": [{
			"name": "slack.receiver",
			"grafana_managed_receiver_configs": [{
				"settings": {
					"recipient": "#unified-alerting-test-but-updated"
				},
				"secureFields": {
					"url": true
				},
				"type": "slack",
				"sendReminder": true,
				"name": "slack.receiver",
				"disableResolveMessage": false
			}]
		}]
	}
}
`
		resp := postRequest(t, alertConfigURL, payload, http.StatusAccepted) // nolint
		require.JSONEq(t, `{"message": "configuration created"}`, getBody(t, resp.Body))
	}

	// The secure settings must be present
	{
		resp := getRequest(t, alertConfigURL, http.StatusOK) // nolint
		require.JSONEq(t, `
{
	"template_files": {},
	"alertmanager_config": {
		"route": {
			"receiver": "slack.receiver"
		},
		"templates": null,
		"receivers": [{
			"name": "slack.receiver",
			"grafana_managed_receiver_configs": [{
				"id": 0,
				"uid": "",
				"name": "slack.receiver",
				"type": "slack",
				"isDefault": false,
				"sendReminder": true,
				"disableResolveMessage": false,
				"frequency": "",
				"created": "0001-01-01T00:00:00Z",
				"updated": "0001-01-01T00:00:00Z",
				"settings": {
					"recipient": "#unified-alerting-test-but-updated"
				},
				"secureFields": {
					"url": true
				}
			}]
		}]
	}
}
`, getBody(t, resp.Body))
	}
}
