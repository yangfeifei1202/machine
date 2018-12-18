package oci

import (
	"github.com/docker/machine/libmachine/drivers"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/core"
)

// Driver represents Oci Docker Machine Driver.
type Driver struct {
	*drivers.BaseDriver

	tenancy string

	CompartmentName string
	DisplayName string
	AvailabilityDomain string
	ImageName string
    Shape string

	// default retry policy will retry on non-200 response
	RequestMetadata common.RequestMetadata

	InstanceID string

	DockerPort int
}

const (

	defaultOciShape            = "VM.Standard2.1"
	defaultOciLocation        = ""
	defaultSSHUser              = "" 
	defaultDockerPort           = 2376 //？
	defaultOciImage           = ""
	defaultOciVNet            = ""
	defaultOciSubnet          = ""
	defaultOciSubnetPrefix    = ""
	defaultStorageType          = string(storage.StandardLRS)

)

const (
	driverName               = "oci"
	ipRange                  = "0.0.0.0/0"
	machineSecurityGroupName = "rancher-nodes"
	machineTag               = "rancher-nodes"
	
)

// NewDriver returns a new driver instance.
func NewDriver(hostName, storePath string) drivers.Driver {
	d := &Driver{
		BaseDriver: &drivers.BaseDriver{
			SSHUser:     defaultSSHUser,
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
	return d
}

//Stop issues a power off for the virtual machine instance.
func (d *Driver) Stop() error  {
	fmt.Println("Stoping inst03")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId=d.InstanceID
	request.Action=core.InstanceActionActionEnum("STOP")

	_, err = c.InstanceAction(ctx , request) 
	helpers.FatalIfError(err)

	return 
}

//Start issues a power on for the virtual machine instance.
func (d *Driver) Start() error  {
	fmt.Println("Starting instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId=d.InstanceID
	request.Action=core.InstanceActionActionEnum("START")

	_, err = c.InstanceAction(ctx , request) 
	helpers.FatalIfError(err)

	return err
}
// GetState returns the state of the virtual machine role instance.
func (d *Driver) GetState() (state.State, error) {
	fmt.Println("Getting state of instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.GetInstanceRequest{}
	equest.InstanceId=d.InstanceID

	response, err := c.GetInstance(ctx , request) 
	lifecycleState := response.Instance.LifecycleState
	machineState := machineStateForLifecycleState(lifecycleState)
	log.Debugf("Determined Oci LifecycleState=%q, docker-machine state=%q",
	lifecycleState, machineState)
	return machineState, nil
}

// DriverName returns the name of the driver.
func (d *Driver) DriverName() string { return driverName }

// GetCreateFlags returns list of create flags driver accepts.
func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		
	}
}

// SetConfigFromFlags initializes driver values from the command line values
// and checks if the arguments have values.
func (d *Driver) SetConfigFromFlags(fl drivers.DriverOptions) error {
	// Initialize driver context for machine


	return nil
}

// PreCreateCheck validates if driver values are valid to create the machine.
func (d *Driver) PreCreateCheck() (err error) {

	// Validate if firewall rules can be read correctly


	// Check if virtual machine exists. An existing virtual machine cannot be updated.


	// NOTE(ahmetalpbalkan) we could have done more checks here but Azure often
	// returns meaningful error messages and it would be repeating the backend
	// logic on the client side. Some examples:
	//   - Deployment of a machine to an existing Virtual Network fails if
	//     virtual network is in a different region.
	//   - Changing IP Address space of a subnet would fail if there are machines
	//     running in the Virtual Network.
	log.Info("Completed machine pre-create checks.")
	return nil
}

// Create creates the virtual machine.
func (d *Driver) Create() error {
	log.Debug("Prepareing launching request.")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()
	request := core.LaunchInstanceRequest{}

	request.DisplayName=d.DisplayName
	request.AvailabilityDomain = d.AvailableDomain
	request.Shape = d.Shape

	compartmentID, err := d.getCompartmentID(ctx, common.DefaultConfigProvider(), d.CompartmentName)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	request.CompartmentId = compartmentId

	imageid, err := getImageID(ctx, common.DefaultConfigProvider(), d.ImageName)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	request.ImageId = imageid

	request.RequestMetadata = helpers.GetRequestMetadataWithDefaultRetryPolicy()

	createResp, err := c.LaunchInstance(ctx, request)
	fmt.Println("Launching Oci instance.")

	// should retry condition check which returns a bool value indicating whether to do retry or not
	// it checks the lifecycle status equals to Running or not for this case
	shouldRetryFunc := func(r common.OCIOperationResponse) bool {
		if converted, ok := r.Response.(core.GetInstanceResponse); ok {
			return converted.LifecycleState != core.InstanceLifecycleStateRunning
		}
		return true
	}
	// create get instance request with a retry policy which takes a function
	// to determine shouldRetry or not
	pollingGetRequest := core.GetInstanceRequest{
		InstanceId:      createResp.Instance.Id,
		RequestMetadata: helpers.GetRequestMetadataWithCustomizedRetryPolicy(shouldRetryFunc),
	}
	_, pollError := c.GetInstance(ctx, pollingGetRequest)
	helpers.FatalIfError(pollError)

	fmt.Println("Oci instance launched")

	return nil
}

// Remove deletes the virtual machine and resources associated to it.
func (d *Driver) Remove() error {
	// NOTE In Oci, there is no remove option for virtual
	// machines, terminate is the closest option.
	log.Debug("Oci does not implement remove. Calling terminate instead.")
	request := core.TerminateInstanceRequest{
		RequestMetadata: helpers.GetRequestMetadataWithDefaultRetryPolicy(),
	}
	request.InstanceId=d.InstanceID

	_, err := c.TerminateInstance(ctx, request)
	helpers.FatalIfError(err)

	
	fmt.Println("terminating instance")

	// should retry condition check which returns a bool value indicating whether to do retry or not
	// it checks the lifecycle status equals to Terminated or not for this case
	shouldRetryFunc := func(r common.OCIOperationResponse) bool {
		if converted, ok := r.Response.(core.GetInstanceResponse); ok {
			return converted.LifecycleState != core.InstanceLifecycleStateTerminated
		}
		return true
	}

	pollGetRequest := core.GetInstanceRequest{
		RequestMetadata: helpers.GetRequestMetadataWithCustomizedRetryPolicy(shouldRetryFunc),
	}

	pollGetRequest.InstanceId=d.InstanceID
	_, pollErr := c.GetInstance(ctx, pollGetRequest)
	helpers.FatalIfError(pollErr)
	fmt.Println("instance terminated")
	return err
}

// GetIP returns public IP address or hostname of the machine instance.
func (d *Driver) GetIP() (string, error) {

	log.Debugf("Machine IP address resolved to: %s", d.resolvedIP)
	return d.resolvedIP, nil
}

// GetSSHHostname returns an IP address or hostname for the machine instance.
func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

// GetURL returns a socket address to connect to Docker engine of the machine
// instance.
func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		return "", err
	}

	// That this is not used until machine is
	// actually created and provisioned. By then GetIP() should be returning
	// a non-empty IP address as the VM is already allocated and connected to.
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	u := (&url.URL{
		Scheme: "tcp",
		Host:   net.JoinHostPort(ip, fmt.Sprintf("%d", d.DockerPort)),
	}).String()
	log.Debugf("Machine URL is resolved to: %s", u)
	return u, nil
}

// Restart reboots the virtual machine instance.
func (d *Driver) Restart() error {
	fmt.Println("Restarting instance")
	c, err := core.NewComputeClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)
	ctx := context.Background()

	request := core.InstanceActionRequest{}
	request.InstanceId=d.InstanceID
	request.Action=core.InstanceActionActionEnum("SOFTRESET")

	_, err = c.InstanceAction(ctx , request) 
	helpers.FatalIfError(err)

	return err
}

// Kill stops the virtual machine role instance.
func (d *Driver) Kill() error {
	// NOTE In Oci, there is no kill option for virtual
	// machines, Stop() is the closest option.
	log.Debug("Oci does not implement kill. Calling Stop instead.")
	return d.Stop()
}

// 
func (d *Driver) listCompartments(ctx context.Context, c core.IdentityClient, compartmentID *string) ([]identity.Compartment,error) {
	request := core.ListCompartmentsRequest{
		CompartmentId: compartmentID,
	}

	r, err := c.ListCompartments(ctx, request, tenancy)
	helpers.FatalIfError(err)

	return r.Items, err
}

func (d *Driver) getCompartmentID(ctx context.Context, provider common.ConfigurationProvider, compartmentName string) （*string, error) {
	c, clerr := identity.NewIdentityClientWithConfigurationProvider(provider)
	if clerr != nil {
		fmt.Println("Error:", clerr)
		return nil, clerr
	}
	Compartments := listCompartments(ctx, c, d.tenancy)

	for _, compartment := range Compartments {
		if *compartment.DisplayName == compartmentName {
			// VCN already created, return it
			return compartment.Id, nil
		}
	}
	err := fmt.Errorf("Can't find Compartment with name %s", compartmentName)
	return nil, err
}

// ListImages lists the available images in the specified compartment.
func (d *Driver) listImages(ctx context.Context, c core.ComputeClient, compartmentID *string) ([]core.Image,error) {
	request := core.ListImagesRequest{
		CompartmentId: compartmentID,
	}

	r, err := c.ListImages(ctx, request)
	helpers.FatalIfError(err)

	return r.Items, err
}


func (d *Driver) getImageID(ctx context.Context, provider common.ConfigurationProvider, imageName string) (*string,error){
	c, clerr := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
	if clerr != nil {
		fmt.Println("Error:", clerr)
	}
	Images := listImages(ctx, c, d.tenancy)

	for _, image := range Images {
		if *image.DisplayName == imageName {
			// VCN already created, return it
			return image.Id, nil
		}
	}
	err := fmt.Errorf("Can't find Image with name %s", imageName)
	return nil, err
}