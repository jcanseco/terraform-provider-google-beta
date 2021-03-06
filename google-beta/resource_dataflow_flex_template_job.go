package google

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"google.golang.org/api/googleapi"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	dataflow "google.golang.org/api/dataflow/v1b3"
)

// NOTE: resource_dataflow_flex_template currently does not support updating existing jobs.
// Changing any non-computed field will result in the job being deleted (according to its
// on_delete policy) and recreated with the updated parameters.

// resourceDataflowFlexTemplateJob defines the schema for Dataflow FlexTemplate jobs.
func resourceDataflowFlexTemplateJob() *schema.Resource {
	return &schema.Resource{
		Create: resourceDataflowFlexTemplateJobCreate,
		Read:   resourceDataflowFlexTemplateJobRead,
		Update: resourceDataflowFlexTemplateJobUpdate,
		Delete: resourceDataflowFlexTemplateJobDelete,
		Schema: map[string]*schema.Schema{

			"container_spec_gcs_path": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"region": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Computed:    true,
				Description: `The region in which the created job should run.`,
			},

			"on_delete": {
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice([]string{"cancel", "drain"}, false),
				Optional:     true,
				Default:      "cancel",
			},

			"labels": {
				Type:             schema.TypeMap,
				Optional:         true,
				DiffSuppressFunc: resourceDataflowJobLabelDiffSuppress,
				ForceNew:         true,
				// TODO add support for labels when the API supports it
				Deprecated: "Deprecated until the API supports this field",
			},

			"parameters": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},

			"project": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"job_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"state": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
		UseJSONNumber: true,
	}
}

// resourceDataflowFlexTemplateJobCreate creates a Flex Template Job from TF code.
func resourceDataflowFlexTemplateJobCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	userAgent, err := generateUserAgentString(d, config.userAgent)
	if err != nil {
		return err
	}

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	region, err := getRegion(d, config)
	if err != nil {
		return err
	}

	request := dataflow.LaunchFlexTemplateRequest{
		LaunchParameter: &dataflow.LaunchFlexTemplateParameter{
			ContainerSpecGcsPath: d.Get("container_spec_gcs_path").(string),
			JobName:              d.Get("name").(string),
			Parameters:           expandStringMap(d, "parameters"),
		},
	}

	response, err := config.NewDataflowClient(userAgent).Projects.Locations.FlexTemplates.Launch(project, region, &request).Do()
	if err != nil {
		return err
	}

	job := response.Job
	d.SetId(job.Id)
	if err := d.Set("job_id", job.Id); err != nil {
		return fmt.Errorf("Error setting job_id: %s", err)
	}

	return resourceDataflowFlexTemplateJobRead(d, meta)
}

// resourceDataflowFlexTemplateJobRead reads a Flex Template Job resource.
func resourceDataflowFlexTemplateJobRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	userAgent, err := generateUserAgentString(d, config.userAgent)
	if err != nil {
		return err
	}

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	region, err := getRegion(d, config)
	if err != nil {
		return err
	}

	jobId := d.Id()

	job, err := resourceDataflowJobGetJob(config, project, region, userAgent, jobId)
	if err != nil {
		return handleNotFoundError(err, d, fmt.Sprintf("Dataflow job %s", jobId))
	}

	if err := d.Set("state", job.CurrentState); err != nil {
		return fmt.Errorf("Error setting state: %s", err)
	}
	if err := d.Set("name", job.Name); err != nil {
		return fmt.Errorf("Error setting name: %s", err)
	}
	if err := d.Set("project", project); err != nil {
		return fmt.Errorf("Error setting project: %s", err)
	}
	if err := d.Set("labels", job.Labels); err != nil {
		return fmt.Errorf("Error setting labels: %s", err)
	}

	if _, ok := dataflowTerminalStatesMap[job.CurrentState]; ok {
		log.Printf("[DEBUG] Removing resource '%s' because it is in state %s.\n", job.Name, job.CurrentState)
		d.SetId("")
		return nil
	}

	return nil
}

// resourceDataflowFlexTemplateJobUpdate is a blank method to enable updating
// the on_delete virtual field
func resourceDataflowFlexTemplateJobUpdate(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceDataflowFlexTemplateJobDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	userAgent, err := generateUserAgentString(d, config.userAgent)
	if err != nil {
		return err
	}

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	region, err := getRegion(d, config)
	if err != nil {
		return err
	}

	id := d.Id()

	requestedState, err := resourceDataflowJobMapRequestedState(d.Get("on_delete").(string))
	if err != nil {
		return err
	}

	// Retry updating the state while the job is not ready to be canceled/drained.
	err = resource.Retry(time.Minute*time.Duration(15), func() *resource.RetryError {
		// To terminate a dataflow job, we update the job with a requested
		// terminal state.
		job := &dataflow.Job{
			RequestedState: requestedState,
		}

		_, updateErr := resourceDataflowJobUpdateJob(config, project, region, userAgent, id, job)
		if updateErr != nil {
			gerr, isGoogleErr := updateErr.(*googleapi.Error)
			if !isGoogleErr {
				// If we have an error and it's not a google-specific error, we should go ahead and return.
				return resource.NonRetryableError(updateErr)
			}

			if strings.Contains(gerr.Message, "not yet ready for canceling") {
				// Retry cancelling job if it's not ready.
				// Sleep to avoid hitting update quota with repeated attempts.
				time.Sleep(5 * time.Second)
				return resource.RetryableError(updateErr)
			}

			if strings.Contains(gerr.Message, "Job has terminated") {
				// Job has already been terminated, skip.
				return nil
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Wait for state to reach terminal state (canceled/drained/done)
	_, ok := dataflowTerminalStatesMap[d.Get("state").(string)]
	for !ok {
		log.Printf("[DEBUG] Waiting for job with job state %q to terminate...", d.Get("state").(string))
		time.Sleep(5 * time.Second)

		err = resourceDataflowFlexTemplateJobRead(d, meta)
		if err != nil {
			return fmt.Errorf("Error while reading job to see if it was properly terminated: %v", err)
		}
		_, ok = dataflowTerminalStatesMap[d.Get("state").(string)]
	}

	// Only remove the job from state if it's actually successfully canceled.
	if _, ok := dataflowTerminalStatesMap[d.Get("state").(string)]; ok {
		log.Printf("[DEBUG] Removing dataflow job with final state %q", d.Get("state").(string))
		d.SetId("")
		return nil
	}
	return fmt.Errorf("Unable to cancel the dataflow job '%s' - final state was %q.", d.Id(), d.Get("state").(string))
}
