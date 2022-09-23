package kafkas

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"redhat.com/rhoas/rhoas-terraform-provider/m/rhoas"
	rhoasAPI "redhat.com/rhoas/rhoas-terraform-provider/m/rhoas/api"
	"redhat.com/rhoas/rhoas-terraform-provider/m/rhoas/utils"
)

var (
	testAccProviders map[string]*schema.Provider
	testAccRHOAS     *schema.Provider
)

const (
	kafkaID   = "test_kafka"
	kafkaPath = "rhoas_kafka.test_kafka"
)

func init() {
	testAccRHOAS = rhoas.Provider()
	testAccProviders = map[string]*schema.Provider{
		"rhoas": testAccRHOAS,
	}
}

// TestAccRHOASKafka_Basic checks that this provider is able to spin up a
// Kafka cluster and then destroy it.
func TestAccRHOASKafka_Basic(t *testing.T) {
	randomName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKafkaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKafkaBasic(kafkaID, randomName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKafkaExists(kafkaPath),
					resource.TestCheckResourceAttr(
						kafkaPath, "name", randomName),
					// TODO: Add more checks?
				),
			},
		},
	})

}

// TestAccRHOASKafka_Update checks that this provider is able to spin up a
// Kafka cluster and then update it. Finnally, it destroys the resource.
func TestAccRHOASKafka_Update(t *testing.T) {
	randomName := fmt.Sprintf("test-%s", randomString(10))
	kafkaPath := fmt.Sprintf("rhoas_kafka.%s", randomName)
	preName := fmt.Sprintf("%s-pre", randomName)
	postName := fmt.Sprintf("%s-post", randomName)

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKafkaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKafkaBasic(kafkaID, preName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKafkaExists(kafkaPath),
					resource.TestCheckResourceAttr(
						kafkaPath, "name", preName),
					// TODO: Add more checks?
				),
			},
			{
				Config: testAccKafkaBasic(kafkaID, postName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKafkaExists(kafkaPath),
					resource.TestCheckResourceAttr(
						kafkaPath, "name", postName),
					// TODO: Add more checks?
				),
			},
		},
	})
}

// TestAccRHOASKafka_Error checks that this provider returns an error if
// some field is missconfigured
func TestAccRHOASKafka_Error(t *testing.T) {
	randomName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKafkaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKafkaWithCloudProvider(
					kafkaID, randomName, "not-a-cloud-provider"),
				ExpectError: regexp.MustCompile(
					"provider not-a-cloud-provider is not supported, supported providers are:",
				),
			},
		},
	})

}

// testAccPreCheck validates the necessary test API keys exist
// in the testing environment
func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("OFFLINE_TOKEN"); v == "" {
		t.Fatal("OFFLINE_TOKEN must be set for acceptance tests")
	}
}

// testAccCheckKafkaDestroy verifies the Kafka cluster has been destroyed
func testAccCheckKafkaDestroy(s *terraform.State) error {
	// retrieve the connection established in Provider configuration
	api, ok := testAccRHOAS.Meta().(rhoasAPI.Clients)
	if !ok {
		return errors.Errorf("unable to cast %v to rhoasAPI.Clients)", testAccRHOAS.Meta())
	}

	// loop through the resources in state, verifying each widget of type rhoas_kafka is destroyed
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "rhoas_kafka" {
			continue
		}

		// Retrieve the kafka struct by referencing it's state ID for API lookup
		kafka, resp, err := api.KafkaMgmt().GetKafkaById(context.Background(), rs.Primary.ID).Execute()
		if err != nil {
			if err.Error() == "404 Not Found" {
				return nil
			}
			if apiErr := utils.GetAPIError(resp, err); apiErr != nil {
				return apiErr
			}
		}

		return errors.Errorf("expected a 404 but found a kafka instance: %v", kafka)
	}

	return nil
}

func testAccCheckKafkaExists(resource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resource]
		if !ok {
			return fmt.Errorf("Not found: %s", resource)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("No Record ID is set")
		}

		api, ok := testAccRHOAS.Meta().(rhoasAPI.Clients)
		if !ok {
			return errors.Errorf("unable to cast %v to rhoasAPI.Clients)", testAccRHOAS.Meta())
		}
		kafka, resp, err := api.KafkaMgmt().GetKafkaById(context.Background(), rs.Primary.ID).Execute()
		if err != nil {
			if apiErr := utils.GetAPIError(resp, err); apiErr != nil {
				return apiErr
			}
		}

		if *kafka.Status != "ready" {
			return errors.Errorf("error provisioning kafka. Status is %s", *kafka.Status)
		}

		return nil
	}
}

func randomString(length int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

	rand.Seed(time.Now().UnixNano())
	b := make([]rune, length)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))] // #nosec G404
	}
	return string(b)
}

func testAccKafkaBasic(id, name string) string {
	return fmt.Sprintf(`
resource "rhoas_kafka" "%s" {
  name = "%s"
}
`, id, name)
}

func testAccKafkaWithCloudProvider(id, name, cloudProvider string) string {
	return fmt.Sprintf(`
resource "rhoas_kafka" "%s" {
  name = "%s"
  cloud_provider = "%s"
}
`, id, name, cloudProvider)
}

func Test_testAccKafkaBasic(t *testing.T) {
	assert.Equal(
		t, `
resource "rhoas_kafka" "test_id" {
  name = "test_name"
}
`,
		testAccKafkaBasic("test_id", "test_name"),
	)
}
