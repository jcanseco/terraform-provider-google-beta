// ----------------------------------------------------------------------------
//
//     ***     AUTO GENERATED CODE    ***    AUTO GENERATED CODE     ***
//
// ----------------------------------------------------------------------------
//
//     This file is automatically generated by Magic Modules and manual
//     changes will be clobbered when the file is regenerated.
//
//     Please read more about how to change this file in
//     .github/CONTRIBUTING.md.
//
// ----------------------------------------------------------------------------

package google

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataLossPreventionInspectTemplate_dlpInspectTemplateBasicExample(t *testing.T) {
	t.Parallel()

	context := map[string]interface{}{
		"project":       getTestProjectFromEnv(),
		"random_suffix": randString(t, 10),
	}

	vcrTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		ExternalProviders: map[string]resource.ExternalProvider{
			"random": {},
		},
		CheckDestroy: testAccCheckDataLossPreventionInspectTemplateDestroyProducer(t),
		Steps: []resource.TestStep{
			{
				Config: testAccDataLossPreventionInspectTemplate_dlpInspectTemplateBasicExample(context),
			},
			{
				ResourceName:            "google_data_loss_prevention_inspect_template.basic",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"parent"},
			},
		},
	})
}

func testAccDataLossPreventionInspectTemplate_dlpInspectTemplateBasicExample(context map[string]interface{}) string {
	return Nprintf(`
resource "google_data_loss_prevention_inspect_template" "basic" {
	parent = "projects/%{project}"
	description = "My description"
	display_name = "display_name"

	inspect_config {
		info_types {
			name = "EMAIL_ADDRESS"
		}
		info_types {
			name = "PERSON_NAME"
		}
		info_types {
			name = "LAST_NAME"
		}
		info_types {
			name = "DOMAIN_NAME"
		}
		info_types {
			name = "PHONE_NUMBER"
		}
		info_types {
			name = "FIRST_NAME"
		}

		min_likelihood = "UNLIKELY"
		rule_set {
			info_types {
				name = "EMAIL_ADDRESS"
			}
			rules {
				exclusion_rule {
					regex {
						pattern = ".+@example.com"
					}
					matching_type = "MATCHING_TYPE_FULL_MATCH"
				}
			}
		}
		rule_set {
			info_types {
				name = "EMAIL_ADDRESS"
			}
			info_types {
				name = "DOMAIN_NAME"
			}
			info_types {
				name = "PHONE_NUMBER"
			}
			info_types {
				name = "PERSON_NAME"
			}
			info_types {
				name = "FIRST_NAME"
			}
			rules {
				exclusion_rule {
					dictionary {
						word_list {
							words = ["TEST"]
						}
					}
					matching_type = "MATCHING_TYPE_PARTIAL_MATCH"
				}
			}
		}

		rule_set {
			info_types {
				name = "PERSON_NAME"
			}
			rules {
				hotword_rule {
					hotword_regex {
						pattern = "patient"
					}
					proximity {
						window_before = 50
					}
					likelihood_adjustment {
						fixed_likelihood = "VERY_LIKELY"
					}
				}
			}
		}

		limits {
			max_findings_per_item    = 10
			max_findings_per_request = 50
			max_findings_per_info_type {
				max_findings = "75"
				info_type {
					name = "PERSON_NAME"
				}
			}
			max_findings_per_info_type {
				max_findings = "80"
				info_type {
					name = "LAST_NAME"
				}
			}
		}
	}
}
`, context)
}

func testAccCheckDataLossPreventionInspectTemplateDestroyProducer(t *testing.T) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "google_data_loss_prevention_inspect_template" {
				continue
			}
			if strings.HasPrefix(name, "data.") {
				continue
			}

			config := googleProviderConfig(t)

			url, err := replaceVarsForTest(config, rs, "{{DataLossPreventionBasePath}}{{parent}}/inspectTemplates/{{name}}")
			if err != nil {
				return err
			}

			_, err = sendRequest(config, "GET", "", url, nil)
			if err == nil {
				return fmt.Errorf("DataLossPreventionInspectTemplate still exists at %s", url)
			}
		}

		return nil
	}
}
