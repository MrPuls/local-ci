package docker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/MrPuls/local-ci/cmd/archive"
	"github.com/MrPuls/local-ci/cmd/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"io"
	"os"
	"strings"
	"time"
)

type Utils struct {
	Workdir    string
	Image      string
	CacheKey   string
	CacheDirs  []string
	Variables  []string
	Scripts    string
	Volumes    volume.Volume
	VolumeDirs map[string]struct{}
}

func (utils *Utils) resolveWorkdir(block config.StageConfig) {
	if block.Workdir != "" {
		utils.Workdir = block.Workdir
	} else {
		utils.Workdir = "/"
	}
}

func (utils *Utils) resolveImage(block config.StageConfig) {
	utils.Image = block.Image
}

func (utils *Utils) resolveVariables(cfg config.Config, block config.StageConfig) {
	var envVars []string
	// append global vars, skip if var is present on block level
	for k, v := range cfg.GlobalVariables {
		if _, ok := block.Variables[k]; ok {
			continue
		}
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}
	// append block level vars
	for k, v := range block.Variables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	utils.Variables = envVars
}

func (utils *Utils) resolveScripts(block config.StageConfig) {
	utils.Scripts = strings.Join(block.Script, "&&")
}

func (utils *Utils) resolveCache(block config.StageConfig) {
	utils.CacheKey = block.Cache.Key
	for _, dest := range block.Cache.Paths {
		utils.CacheDirs = append(utils.CacheDirs, fmt.Sprintf("%s:%s", utils.CacheKey, utils.Workdir+dest))
	}
	fmt.Printf("Cache dirs: %v\n", utils.CacheDirs)
}

func (utils *Utils) resolveVolumes(ctx context.Context, cli *client.Client) {
	volumes, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, v := range volumes.Volumes {
		fmt.Printf("Inspecting volume: %s\n", v.Name)
		if v.Name == utils.CacheKey {
			fmt.Printf("Volume '%s' already exists\n", v.Name)
			return
		}
	}
	fmt.Printf("Creating volume '%s'\n", utils.CacheKey)
	vlm, cErr := cli.VolumeCreate(ctx, volume.CreateOptions{Name: utils.CacheKey})
	if cErr != nil {
		panic(cErr)
	}
	utils.Volumes = vlm
}

func (utils *Utils) resolveVolumeDir(block config.StageConfig) {
	utils.VolumeDirs = make(map[string]struct{})
	for _, dest := range block.Cache.Paths {
		fmt.Printf("Creating volume directory '%s'\n", dest)
		utils.VolumeDirs[dest] = struct{}{}
	}
}

func ExecuteConfigPipeline(wd string, yamlConf config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	utils := &Utils{}
	defer cancel()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer func(cli *client.Client) {
		err := cli.Close()
		if err != nil {
			panic(err)
		}
	}(cli)

	for blockName, block := range yamlConf.Blocks {
		utils.resolveWorkdir(block)
		utils.resolveImage(block)
		utils.resolveScripts(block)
		utils.resolveCache(block)
		utils.resolveVariables(yamlConf, block)
		utils.resolveVolumes(ctx, cli)
		utils.resolveVolumeDir(block)

		reader, err := cli.ImagePull(ctx, utils.Image, image.PullOptions{})
		fmt.Println("Image is pulled")
		if err != nil {
			panic(err)
		}
		_, errCp := io.Copy(os.Stdout, reader)
		if errCp != nil {
			panic(errCp)
		}

		// TODO: UPD: All works, yay!
		//		Also add cache docs!
		fmt.Println("Trying to create a container!")
		fmt.Printf(
			"Creating a container with config\n Image:%s,\nWorkingDir: %s,\nCmd: %s,\nEnv:%s,\nVolumes:%s\n,",
			utils.Image, utils.Workdir, utils.Scripts, utils.Variables, utils.CacheDirs,
		)
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:      utils.Image,
			WorkingDir: utils.Workdir,
			Cmd:        []string{"/bin/sh", "-c", utils.Scripts},
			Env:        utils.Variables,
			Volumes:    utils.VolumeDirs,
		}, &container.HostConfig{
			//Binds: utils.CacheDirs,
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeVolume,
					Source: utils.CacheKey, // your volume name
					Target: "/.venv",       // where it will be mounted in container
				},
			},
		}, nil, nil, blockName)
		if err != nil {
			panic(err)
		}

		var b bytes.Buffer
		fmt.Println("Trying to create a fs tar!")
		fsErr := archive.CreateFSTar(wd, &b)
		if fsErr != nil {
			panic(fsErr)
		}

		fmt.Println("Trying to copy files to container!")
		errCpCtr := cli.CopyToContainer(ctx, resp.ID, utils.Workdir, &b, container.CopyToContainerOptions{})
		if errCpCtr != nil {
			panic(errCpCtr)
		}

		logs, err := cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{Stream: true, Stdout: true, Stderr: true})
		if err != nil {
			panic(err)
		}

		fmt.Println("Trying to start a container!")
		if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			panic(err)
		}

		fmt.Println("Copying files to container!")
		_, err = io.Copy(os.Stdout, logs.Reader)
		if err != nil && err != io.EOF {
			panic(err)
		}

		statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				panic(err)
			}
		case <-statusCh:
		}

		fmt.Println("Job is done, removing container...")
		delCntErr := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})
		if delCntErr != nil {
			panic(delCntErr)
		}

		logs.Close()

		fmt.Println("All done!")
	}
}
