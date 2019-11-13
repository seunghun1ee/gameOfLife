package main

import (
	"fmt"
	"strconv"
	"strings"
)

func golLogic(world [][]byte, startY int, endY int, startX int, endX int) [][]byte {
	height := endY - startY
	width := endX - startX
	//init result
	result := make([][]byte, height)
	for i := range world {
		result[i] = make([]byte, width)
	}
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			count := 0

			//counts neighbors
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					if world[(y-1+i+height)%height][(x-1+j+width)%width] == 0xFF {
						count++
					}
				}
			}
			//calculating alive or dead
			if world[y][x] == 0xFF {
				count--
				if count < 2 || count > 3 {
					result[y][x] = 0x00
				} else {
					result[y][x] = world[y][x]
				}
			} else {
				if count == 3 {
					result[y][x] = 0xFF
				} else {
					result[y][x] = world[y][x]
				}
			}

		}
	}
	return result
}

func worker (p golParams, world [][]byte, startY int, endY int, startX int, endX int, out chan<- [][]byte) {
	out <- golLogic(world, startY, endY, startX, endX)
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell) {

	// Create the 2D slice to store the world.
	// Create new world here
	world := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	workerChans := make([]chan [][]byte, p.threads)
	for i := range workerChans {
		workerChans[i] = make(chan [][]byte)
	}



	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {
		newWorld := golLogic(world, 0, p.imageHeight, 0, p.imageWidth)
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = newWorld[y][x]
			}
		}
	}

	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight), strconv.Itoa(p.turns)}, "x")
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[y][x]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
