package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

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
		if params.Repository.Repo_name == c.Repository && params.Push_data.Tag == c.Tag {
			go runContainer(c)
			http.Get(params.Callback_url)
		} else {
			log.Println("Image is not being watched:")
			log.Println("IMAGE: " + params.Repository.Repo_name)
			log.Println("TAG: " + params.Push_data.Tag)
		}
	}
}

func runContainer(config Cnf) error {

	// start new docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Println(err)
		return err
	}
	defer cli.Close()

	// login docker hub
	_, err = cli.RegistryLogin(context.Background(), types.AuthConfig{Username: ENV_USERNAME, Password: ENV_PASSWORD})
	if err == nil {
		log.Println(err)
		return err
	}

	// pull new image
	log.Println("Pulling new image")
	out, err := cli.ImagePull(context.Background(), config.Repository+":"+config.Tag, types.ImagePullOptions{Platform: "linux/amd64"})
	if err != nil {
		log.Println(err)
		return err
	}
	defer out.Close()
	io.Copy(os.Stdout, out)

	// scan running containers
	log.Println("Scanning for running containrs")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Println(err)
	}

	// if image is already running, stop it
	for _, container := range containers {
		log.Println(container.Image)
		if container.Image == config.Repository+":"+config.Tag {
			log.Println("Image is not being watched:")
			if err := cli.ContainerStop(context.Background(), container.ID, nil); err != nil {
				log.Println(err)
			}
		}
		log.Println("stopped ", container.Image)
	}

	// create new container
	resp, err := cli.ContainerCreate(context.Background(),
		&dockercontainer.Config{
			Image: config.Repository + ":" + config.Tag,
			ExposedPorts: nat.PortSet{
				nat.Port(config.Containerport + "/tcp"): {},
			},
		},
		&dockercontainer.HostConfig{
			Binds: []string{
				"/var/run/docker.sock:/var/run/docker.sock",
			},
			PortBindings: nat.PortMap{
				nat.Port(config.Containerport + "/tcp"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: config.Hostport}},
			},
		}, nil, nil, "")
	if err != nil {
		panic(err)
	}

	// run new container
	if err := cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	// close client interface
	cli.Close()

	log.Println("Done!")
	return nil
}
