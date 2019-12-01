package main

import (
	"fmt"
	"strconv"
	"strings"
)

type worker struct {
	upper chan byte
	lower chan byte
}

//Allocates 2D slice of byte
func allocateSlice(height int, width int) [][]byte {
	slice := make([][]byte, height)
	for i := range slice {
		slice[i] = make([]byte, width)
	}
	return slice
}

//Assigns the height of each worker
func calculateThreadHeight(p golParams) []int {
	heightSlice := make([]int, p.threads)
	leftover := p.imageHeight % p.threads
	for i := range heightSlice {
		heightSlice[i] = p.imageHeight / p.threads
	}
	if leftover != 0 {
		for j := 0; leftover > 0 && j < p.threads; leftover-- {
			heightSlice[j]++
			j = (j + 1) % p.threads
		}
	}
	return heightSlice
}

//Counts number of each cell's neighbors first then apply Game Of Life logic to threads
func golLogic(start [][]byte) [][]byte {
	height := len(start)
	width := len(start[0])
	//init result
	result := allocateSlice(height, width)

	for y := 1; y < height-1; y++ {
		for x := 0; x < width; x++ {
			count := 0

			//counts neighbors
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					if x-1+j < 0 {
						if start[y-1+i][width-1] == 0xFF {
							count++
						}
					} else if x-1+j >= width {
						if start[y-1+i][0] == 0xFF {
							count++
						}
					} else {
						if start[y-1+i][x-1+j] == 0xFF {
							count++
						}

					}
				}
			}
			//calculating alive or dead
			if start[y][x] == 0xFF { //if the cell is alive
				count-- //excludes itself
				if count < 2 || count > 3 {
					result[y][x] = 0x00
				} else {
					result[y][x] = 0xFF
				}
			} else { //if the cell is dead
				if count == 3 {
					result[y][x] = 0xFF
				} else {
					result[y][x] = 0x00
				}
			}

		}
	}
	return result
}

//Send the world to pgm.go
func outputBoard(p golParams, d distributorChans, world [][]byte, turn int) {
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight), strconv.Itoa(turn)}, "x")
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[y][x]
		}
	}
}

//Count the number of alive cells in 2D slice
func countAlive(p golParams, world [][]byte) []cell {
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
	return finalAlive
}

//After go routines of threads are started, sends the split of the world including initial halos
func sendWorldToWorkers(p golParams, world [][]byte, golWorkerChans []chan byte, golCumulativeThreadHeights []int) {
	//Upper halo
	for x := 0; x < p.imageWidth; x++ {
		for i := range golWorkerChans {
			if i == 0 {
				golWorkerChans[0] <- world[golCumulativeThreadHeights[p.threads-1]-1][x]
			} else {
				golWorkerChans[i] <- world[golCumulativeThreadHeights[i-1]-1][x]
			}

		}
	}
	//mid
	for y := 0; y < golCumulativeThreadHeights[0]; y++ {
		for x := 0; x < p.imageWidth; x++ {
			golWorkerChans[0] <- world[y][x]
		}
	}
	for i := 1; i < p.threads; i++ {
		for y := golCumulativeThreadHeights[i-1]; y < golCumulativeThreadHeights[i]; y++ {
			for x := 0; x < p.imageWidth; x++ {
				golWorkerChans[i] <- world[y][x]
			}
		}
	}

	//Lower halo
	for x := 0; x < p.imageWidth; x++ {
		for i := range golWorkerChans {
			if i == p.threads-1 {
				golWorkerChans[i] <- world[0][x]
			} else {
				golWorkerChans[i] <- world[golCumulativeThreadHeights[i]][x]
			}

		}
	}
}

func removeHaloAndMergeThreads(p golParams, golResultChans []chan [][]byte, golHalos [][][]byte, golNonHalos [][][]byte, golThreadHeights []int, golCumulativeThreadHeights []int) [][]byte {
	newWorld := allocateSlice(p.imageHeight, p.imageWidth)

	//Remove halo
	for i := range golResultChans {
		golHalos[i] = <-golResultChans[i]
		golNonHalos[i] = removeHalo(golHalos[i])
		//Merging threads without halo to new world
		if i == 0 { //case thread 0
			for y := 0; y < golThreadHeights[0]; y++ {
				for x := 0; x < p.imageWidth; x++ {
					newWorld[y][x] = golNonHalos[i][y][x]
				}
			}
		} else { //every other cases
			h := 0
			for y := golCumulativeThreadHeights[i-1]; y < golCumulativeThreadHeights[i]; y++ {
				for x := 0; x < p.imageWidth; x++ {
					newWorld[y][x] = golNonHalos[i][h][x]
				}
				h++
			}
		}
	}
	return newWorld
}

//When exchanging the halos, A type workers send the halo first
func golWorkerA(p golParams, cellChan <-chan byte, out chan<- [][]byte, heightInfo int, workers []worker, workerNumber int) {
	height := heightInfo + 2
	width := p.imageWidth
	thisWorker := workers[workerNumber]
	aboveWorker := workers[(workerNumber-1+p.threads)%p.threads]
	belowWorker := workers[(workerNumber+1)%p.threads]

	//Makes thread with incoming cells
	threadWorld := allocateSlice(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := <-cellChan
			threadWorld[y][x] = cell
		}
	}
	//Applying golLogic for every turn
	for turn := 0; turn < p.turns; turn++ {
		for x := 0; x < p.imageWidth; x++ { //Exchanging halo
			thisWorker.upper <- threadWorld[1][x]
			thisWorker.lower <- threadWorld[height-2][x]
			threadWorld[height-1][x] = <-belowWorker.upper
			threadWorld[0][x] = <-aboveWorker.lower
		}
		threadWorld = golLogic(threadWorld)
	}
	//return the final board state after iterating all turns
	out <- threadWorld
}

//When exchanging the halos, B type workers receives the halo first
func golWorkerB(p golParams, cellChan <-chan byte, out chan<- [][]byte, heightInfo int, workers []worker, workerNumber int) {
	height := heightInfo + 2
	width := p.imageWidth
	thisWorker := workers[workerNumber]
	aboveWorker := workers[(workerNumber-1+p.threads)%p.threads]
	belowWorker := workers[(workerNumber+1)%p.threads]

	//Makes thread with incoming cells
	threadWorld := allocateSlice(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := <-cellChan
			threadWorld[y][x] = cell
		}
	}
	//Applying golLogic for every turn
	for turn := 0; turn < p.turns; turn++ {
		for x := 0; x < p.imageWidth; x++ { //Exchanging halo
			threadWorld[height-1][x] = <-belowWorker.upper
			threadWorld[0][x] = <-aboveWorker.lower
			thisWorker.upper <- threadWorld[1][x]
			thisWorker.lower <- threadWorld[height-2][x]
		}
		threadWorld = golLogic(threadWorld)
	}
	//return the final board state after iterating all turns
	out <- threadWorld
}

func removeHalo(input [][]byte) [][]byte {
	height := len(input) - 2
	width := len(input[0])
	output := allocateSlice(height, width)
	for y := 1; y < len(input)-1; y++ {
		for x := 0; x < width; x++ {
			output[y-1][x] = input[y][x]
		}
	}
	return output
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

	if p.turns != 0 {
		//Calculating thread height
		golThreadHeights := calculateThreadHeight(p)

		//Ignoring threads that has height of 0
		for index := 0; index < p.threads; index++ {
			if golThreadHeights[index] == 0 {
				p.threads = index
				break
			}
		}
		//Initialising cumulative heights for sending a split of the world to threads
		golCumulativeThreadHeights := make([]int, p.threads)
		golCumulativeThreadHeights[0] = golThreadHeights[0]
		for i := 1; i < len(golCumulativeThreadHeights); i++ {
			golCumulativeThreadHeights[i] = golCumulativeThreadHeights[i-1] + golThreadHeights[i]
		}

		//Initialising halo channels
		aboveHaloExchanges := make([]chan byte, p.threads)
		belowHaloExchanges := make([]chan byte, p.threads)
		for i := 0; i < p.threads; i++ {
			aboveHaloExchanges[i] = make(chan byte)
			belowHaloExchanges[i] = make(chan byte)
		}
		workers := make([]worker, p.threads)
		for i := 0; i < p.threads; i++ {
			workers[i] = worker{
				upper: aboveHaloExchanges[i],
				lower: belowHaloExchanges[i]}
		}

		//Init slices of channels and slices of threads
		//Slice of threads after gol logic with halo
		golHalos := make([][][]byte, p.threads)
		//Slice of threads after removing halo
		golNonHalos := make([][][]byte, p.threads)
		//Slice of channel of workers before gol logic
		golWorkerChans := make([]chan byte, p.threads)
		//Slice of channel of workers after gol logic
		golResultChans := make([]chan [][]byte, p.threads)
		for i := range golResultChans {
			golResultChans[i] = make(chan [][]byte, golThreadHeights[i]+2)
		}
		for i := range golWorkerChans {
			golWorkerChans[i] = make(chan byte)
		}

		//Go routine starts here
		//Starting threads
		for i := 0; i < len(golWorkerChans); i += 2 {
			go golWorkerA(p, golWorkerChans[i], golResultChans[i], golThreadHeights[i], workers, i)
			go golWorkerB(p, golWorkerChans[i+1], golResultChans[i+1], golThreadHeights[i+1], workers, i+1)
		}

		//send the split of the world to threads
		sendWorldToWorkers(p, world, golWorkerChans, golCumulativeThreadHeights)
		//Update the world after merging threads
		world = removeHaloAndMergeThreads(p, golResultChans, golHalos, golNonHalos, golThreadHeights, golCumulativeThreadHeights)
	}

	//returns number of alive cells
	finalAlive := countAlive(p, world)
	//Sends final world board to pgm.go
	outputBoard(p, d, world, p.turns)

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
