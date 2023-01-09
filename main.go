package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Cnf struct {
	Repository    string `json:"Repository"`
	Image         string `json:"Image"`
	Tag           string `json:"Tag"`
	Containerport string `json:"Containerport"`
	Hostport      string `json:"Hostport"`
}

type DockerWebhookRequest struct {
	Callback_url string
	Push_data    struct {
		Pushed_at int    `json:"Pushed_at"`
		Pusher    string `json:"Pusher"`
		Tag       string `json:"Tag"`
	}
	Repository struct {
		Comment_count    int    `json:"Comment_count"`
		Date_created     int    `json:"Date_created"`
		Description      string `json:"Description"`
		Dockerfile       string `json:"Dockerfile"`
		Full_description string `json:"Full_description"`
		Is_official      bool   `json:"Is_official"`
		Is_private       bool   `json:"Is_private"`
		Is_trusted       bool   `json:"Is_trusted"`
		Name             string `json:"Name"`
		Namespace        string `json:"Namespace"`
		Owner            string `json:"Owner"`
		Repo_name        string `json:"Repo_name"`
		Repo_url         string `json:"Repo_url"`
		Star_count       int    `json:"Star_count"`
		Status           string `json:"Status"`
	}
}

// global configuration variables
var ENV_WEBHOOK_TOKEN, ENV_SERVICESYNC_PORT, ENV_USERNAME, ENV_PASSWORD string
var Config []Cnf

func main() {

	// read environment variables
	ENV_SERVICESYNC_PORT = os.Getenv("ENV_SERVICESYNC_PORT") // port on which to listen for requests
	ENV_WEBHOOK_TOKEN = os.Getenv("ENV_WEBHOOK_TOKEN")       //docker hub webhook token
	ENV_USERNAME = os.Getenv("ENV_USERNAME")                 //docker hub username
	ENV_PASSWORD = os.Getenv("ENV_PASSWORD")                 //docker hub password

	log.Println("Reading config...")
	// read container configs
	file, _ := os.ReadFile("config.json")
	_ = json.Unmarshal([]byte(file), &Config)

	// start webserver
	log.Println("Starting webserver...")
	http.HandleFunc("/webhooks/"+ENV_WEBHOOK_TOKEN, syncDockerService)
	http.ListenAndServe(":"+ENV_SERVICESYNC_PORT, nil)
}

func syncDockerService(w http.ResponseWriter, req *http.Request) {

	// read params
	var params DockerWebhookRequest
	body, _ := ioutil.ReadAll(req.Body)
	json.Unmarshal(body, &params)
	log.Println("Incoming request found for image: " + params.Repository.Repo_name + ":" + params.Push_data.Tag)

	for _, c := range Config {
		if params.Push_data.Tag == c.Tag {
			go runContainer(c)
			http.Get(params.Callback_url)
		}
	}
}

func runContainer(config Cnf) error {

	// start new docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Println("Err: Newclient: ", err)
		return err
	}
	defer cli.Close()

	authConfig := types.AuthConfig{
		Username: ENV_USERNAME,
		Password: ENV_PASSWORD,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	// pull new image
	log.Printf("Pulling new image '%s:%s'\n", config.Repository, config.Tag)
	out, err := cli.ImagePull(context.Background(), config.Repository+":"+config.Tag, types.ImagePullOptions{RegistryAuth: authStr, Platform: "linux/amd64"})
	if err != nil {
		log.Println("Err: ImagePull: ", err)
		return err
	}
	defer out.Close()
	io.Copy(os.Stdout, out)

	// scan running containers
	log.Println("Start scan running containers.")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Println("Running containers: ", err)
	}

	log.Println("Checking if port is already in use.")
	// if port is already in use, stop container using it.
	for _, container := range containers {
		for _, p := range container.Ports {
			hp, _ := strconv.Atoi(config.Hostport)
			if p.PublicPort == uint16(hp) {
				log.Printf("Port %d, is being used by %s\n", p.PublicPort, container.Image)
				err := cli.ContainerStop(context.Background(), container.ID, nil)
				if err != nil {
					log.Println("Err: ContainerStop: ", err)
					return err
				}
				log.Printf("Stopped %s. Hostport %d is now free.\n", container.Image, p.PublicPort)
			}
		}
	}

	// create new container
	log.Println("Creating new container")
	resp, err := cli.ContainerCreate(context.Background(),
		&dockercontainer.Config{
			Image: config.Repository + ":" + config.Tag,
			ExposedPorts: nat.PortSet{
				nat.Port(config.Containerport + "/tcp"): struct{}{},
			},
		},
		&dockercontainer.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port(config.Containerport + "/tcp"): []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: config.Hostport,
					},
				},
			},
		}, nil, nil, "")

	if err != nil {
		log.Println("Err: Creating new container: ", err)
		return err
	}

	// run new container
	log.Println("Starting new container.")
	err = cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Println("Err: Starting new container: ", err)
		return err
	}

	// close client interface
	cli.Close()

	log.Println("Done!")
	return nil
}
