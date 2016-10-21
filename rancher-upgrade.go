package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/client"
	"strings"
	"sync"
	"time"
)

var (
	ACCESSKEY   = flag.String("accesskey", "", "Rancher API accesskey.")
	SECRETKEY   = flag.String("secretkey", "", "Rancher API secretKey.")
	SERVER      = flag.String("url", "http://localhost:8080", "URL endpoint of the Rancher API.")
	SERVICEMAP  map[string]string
	SERVICES    []string
	IMAGEPREFIX = flag.String("image-prefix", "", "URL for registry plus the repo and any other image prefix.")
	IMAGETAG    string
	PARALLELISM = flag.Int("parallelism", 5, "Number of concurrent processes. Defaults to 5.")
	WG          sync.WaitGroup
)

func main() {
	log.Info(log.GetLevel().String())
	upgradeServices(*IMAGEPREFIX, IMAGETAG, SERVICES)
}

func upgradeServicesConcurrent(prefix, tag string, serviceChan chan string) {
	// Defer the wait group decrement so it is guaranteed to take place.
	defer WG.Done()
	for service := range serviceChan {
		imageUuid := prefix + service + tag
		log.Infof("Upgrading %s to %s\n", service, imageUuid)
		upgradeServiceImage(service, imageUuid)
	}
}

func upgradeServices(prefix, tag string, services []string) {
	serviceChan := make(chan string, cap(services))
	for _, service := range services {
		log.Debugf("inserting service %s", service)
		serviceChan <- service
	}
	// We close the channel to *SENDING* here. Receiving can still take place and will not block in `for range` loops.
	close(serviceChan)
	for i := 0; i < *PARALLELISM; i++ {
		WG.Add(1)
		go upgradeServicesConcurrent(prefix, tag, serviceChan)
	}
	WG.Wait()
}

func upgradeServiceImage(serviceName, image string) {
	if err := actionAvailable("upgrade", serviceName); err != nil {
		log.Error(err)
		return
	}
	err := doUpgrade(serviceName, image)
	if err != nil {
		log.Errorf("Error trying to upgrade.\n%s", err)
		return
	}
	for err := actionAvailable("finishupgrade", serviceName); err != nil; err = actionAvailable("finishupgrade", serviceName) {
		time.Sleep(1 * time.Second)
		log.Debugf(".")
	}
	doFinishUpgrade(serviceName)
}

func actionAvailable(action, service string) error {
	client, err := getNewClient()
	if err != nil {
		// If we can't connect then we're calling the action unavailable.
		log.Error(err)
		return &actionAvailableError{action, service}
	}
	if SERVICEMAP[service] == "" {
		return &actionAvailableError{action, service}
	}
	s, err := client.Service.ById(SERVICEMAP[service])
	if err != nil {
		log.Error(err)
		return &actionAvailableError{action, service}
	}
	_, ok := s.Resource.Actions[action]
	if !ok {
		return &actionAvailableError{action, service}
	}
	return nil
}

func doFinishUpgrade(service string) error {
	if err := actionAvailable("finishupgrade", service); err != nil {
		return &upgradeError{"finishupgrade", service, err}
	}
	log.Infof("Finishing Upgrade on %s.", service)
	client, err := getNewClient()
	if err != nil {
		return &upgradeError{"finishupgrade", service, err}
	}
	if SERVICEMAP[service] == "" {
		return &actionAvailableError{action, service}
	}
	s, err := client.Service.ById(SERVICEMAP[service])
	if err != nil {
		return &upgradeError{"finishupgrade", service, err}
	}
	_, err = client.Service.ActionFinishupgrade(s)
	if err != nil {
		return &upgradeError{"finishupgrade", service, err}
	}
	return nil
}

func doUpgrade(serviceName, image string) error {
	if err := actionAvailable("upgrade", serviceName); err != nil {
		return &actionAvailableError{"upgrade", serviceName}
	}
	log.Infof("Upgrading Service %s.", serviceName)
	client, err := getNewClient()
	if err != nil {
		return &upgradeError{"getNewClient", serviceName, err}
	}
	// Get Service object
	if SERVICEMAP[serviceName] == "" {
		return &actionAvailableError{action, serviceName}
	}
	service, err := client.Service.ById(SERVICEMAP[serviceName])
	if err != nil {
		return &upgradeError{"client.Service.ById", serviceName, err}
	}

	// Update settings
	service.Upgrade.InServiceStrategy.StartFirst = true
	service.Upgrade.InServiceStrategy.LaunchConfig.ImageUuid = "docker:" + image

	// Perform Upgrade
	service, err = client.Service.ActionUpgrade(service, service.Upgrade)
	if err != nil {
		return &upgradeError{"client.Service.ActionUpgrade", serviceName, err}
	}
	return nil
}

func getNewClient() (*rancher.RancherClient, error) {
	var client, err = rancher.NewRancherClient(&rancher.ClientOpts{Url: *SERVER, AccessKey: *ACCESSKEY, SecretKey: *SECRETKEY})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func init() {
	var services string
	flag.StringVar(&services, "services", "", "Comma serparated list of services to upgrade.")
	var tag string
	flag.StringVar(&tag, "tag", "latest", "Tag to use for the images.")
	logLevel := flag.String("log", "Info", "Log level. Defaults to Info.")
	flag.Parse()
	InitializeLogging(*logLevel)
	SERVICES = strings.Split(services, ",")
	if !strings.HasPrefix(tag, ":") {
		tag = ":" + tag
	}
	IMAGETAG = tag
	// Do this after parsing flags since it uses them...
	var err error
	SERVICEMAP, err = createServiceMap()
	if err != nil {
		log.Fatalf("Failed creating the service map.\n%s", err)
	}
}

func createServiceMap() (map[string]string, error) {
	client, err := getNewClient()
	if err != nil {
		return nil, err
	}
	var lOpts rancher.ListOpts
	serviceCollection, err := client.Service.List(&lOpts)
	if err != nil {
		return nil, err
	}

	var serviceMap = make(map[string]string)
	for _, service := range serviceCollection.Data {
		serviceMap[service.Name] = service.Id
	}
	return serviceMap, nil
}

func InitializeLogging(logLevel string) {
	switch logLevel {
	case "panic":
		log.SetLevel(log.PanicLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		fallthrough
	default:
		log.SetLevel(log.InfoLevel)
	}
}
