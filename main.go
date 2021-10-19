/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"gocv.io/x/gocv"

	"github.com/FlowingSPDG/ndi-go"
)

const (
	ndiLibName    = "Processing.NDI.Lib.x64.dll"
	ndiSourceName = "FL-9900K (Test Pattern)"
)

func initializeNDI() {
	libDir := os.Getenv("NDI_RUNTIME_DIR_V5")
	if libDir == "" {
		log.Fatalln("ndi sdk is not installed")
	}

	if err := ndi.LoadAndInitialize(path.Join(libDir, ndiLibName)); err != nil {
		log.Fatalln("Failed to initialize NDI library", err)
	}
}

func main() {
	// Init NDI
	initializeNDI()
	defer ndi.DestroyAndUnload()

	// Init GUI
	window := gocv.NewWindow("NDI " + ndiSourceName)
	defer window.Close()
	window.ResizeWindow(1920, 1080)

	pool := ndi.NewObjectPool()
	findSettings := pool.NewFindCreateSettings(true, "", "")
	findInst := ndi.NewFindInstanceV2(findSettings)
	if findInst == nil {
		log.Fatalln("could not create finder")
	}

	var recvInst *ndi.RecvInstance

	log.Println("Searching for NDI sources...")

	for recvInst == nil {
		srcs := findInst.GetCurrentSources()
		for i, source := range srcs {
			name := source.Name()
			log.Printf("Got source %d : %s(%s)\n", i, source.Name(), source.Address())
			if name == ndiSourceName {
				addr := source.Address()
				recvSettings := ndi.NewRecvCreateSettings()
				recvSettings.SourceToConnectTo = *source

				recvInst = ndi.NewRecvInstanceV2(recvSettings)

				if recvInst == nil {
					log.Printf("unable to connect to %s, %s\n", name, addr)
					continue
				}

				fmt.Printf("Connected to %s, %s\n", name, addr)

				findInst.Destroy()
				pool.Release(findSettings)
				break
			}
			time.Sleep(time.Second)
		}
	}

	defer recvInst.Destroy()

	if !recvInst.SetTally(&ndi.Tally{OnProgram: true, OnPreview: true}) {
		log.Println("could not set tally")
	}

	for {
		i, err := recvInst.GetNumConnections(1000)
		if err != nil {
			log.Println("Getnum err", err)
			return
		}
		fmt.Println("connections..", i)
		if i != 0 {
			break
		}
	}

	log.Println("Reading video...")

	for {
		var (
			vf ndi.VideoFrameV2
			af ndi.AudioFrameV2
			mf ndi.MetadataFrame
		)

		vf.SetDefault()
		af.SetDefault()
		mf.SetDefault()

		ft := recvInst.CaptureV2(&vf, &af, &mf, 1000)
		switch ft {
		case ndi.FrameTypeNone:
			log.Println("FrameTypeNone")
		case ndi.FrameTypeVideo:
			log.Println("FrameTypeVideo")
			log.Printf("Received VideoFrame : %#v\n", vf)
			d := vf.ReadData()

			im, err := gocv.NewMatFromBytes(int(vf.Xres), int(vf.Yres), gocv.MatTypeCV8UC4, d)
			if err != nil {
				log.Fatalln("Failed to convert mat from bytes:", err)
			}
			window.IMShow(im)
			window.WaitKey(1)
			recvInst.FreeVideoV2(&vf)
		case ndi.FrameTypeAudio:
			log.Println("FrameTypeAudio")
			log.Printf("Received AudioFrame : %#v\n", af)
			recvInst.FreeAudioV2(&af)
		case ndi.FrameTypeMetadata:
			log.Println("FrameTypeMetadata")
			log.Printf("Received MetaData : %#v\n", mf)
			recvInst.FreeMetadataV2(&mf)
		case ndi.FrameTypeStatusChange:
			log.Println("FrameTypeStatusChange")
		default:
			log.Println("Unknown frame type!")
		}
	}
}
