package main

import (
	"fmt"
	"strconv"
	"strings"
)

func allocateSlice(height int, width int) [][]byte {
	slice := make([][]byte, height)
	for i := range slice {
		slice[i] = make([]byte, width)
	}
	return slice
}

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
					if start[y-1+i][(x-1+j+width)%width] == 0xFF {
						count++
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
func outputBoard(p golParams, d distributorChans, world [][]byte, turn int) {
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight), strconv.Itoa(turn)}, "x")
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[y][x]
		}
	}
}
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

func worker(p golParams, cellChan <-chan byte, out chan<- [][]byte, heightInfo int) {
	height := heightInfo + 2
	width := p.imageWidth

	//Makes thread with incoming cells
	start := allocateSlice(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := <-cellChan
			start[y][x] = cell
		}
	}
	out <- golLogic(start)
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
	newWorld := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
		newWorld[i] = make([]byte, p.imageWidth)
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
				newWorld[y][x] = val
			}
		}
	}
	//Calculating thread height
	golThreadHeights := calculateThreadHeight(p)
	for a := range golThreadHeights {
		fmt.Printf("Thread %d has height %d\n", a, golThreadHeights[a])
	}
	golCumulativeThreadHeights := make([]int, p.threads)
	golCumulativeThreadHeights[0] = golThreadHeights[0]
	fmt.Printf("Thread %d has  cumulative height %d\n", 0, golCumulativeThreadHeights[0])
	for i := 1; i < len(golCumulativeThreadHeights); i++ {
		golCumulativeThreadHeights[i] = golCumulativeThreadHeights[i-1] + golThreadHeights[i]
		fmt.Printf("Thread %d has  cumulative height %d\n", i, golCumulativeThreadHeights[i])
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

	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {

		//Go routine starts here
		for i := range golWorkerChans {
			go worker(p, golWorkerChans[i], golResultChans[i], golThreadHeights[i])
		}

		//Upper halo
		for x := 0; x < p.imageWidth; x++ {
			for i := range golWorkerChans {
				golWorkerChans[i] <- world[golCumulativeThreadHeights[((i-1)+p.threads)%p.threads]-1][x]
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
				golWorkerChans[i] <- world[golCumulativeThreadHeights[i]%p.imageHeight][x]
			}
		}

		//Remove halo
		for i := range golResultChans {
			golHalos[i] = <-golResultChans[i]
			golNonHalos[i] = removeHalo(golHalos[i])
			//Passing threads without halo to new world
			if i == 0 {
				for y := 0; y < golThreadHeights[0]; y++ {
					for x := 0; x < p.imageWidth; x++ {
						newWorld[y][x] = golNonHalos[i][y][x]
					}
				}
			} else {
				h := 0
				for y := golCumulativeThreadHeights[i-1]; y < golCumulativeThreadHeights[i]; y++ {
					for x := 0; x < p.imageWidth; x++ {
						newWorld[y][x] = golNonHalos[i][h][x]
					}
					h++
				}
			}
		}
		//Updating the world with new world
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = newWorld[y][x]
			}
		}

		//Keyboard input section
		//r: input signal from rune channel
		//t: 2 seconds time signal from bool channel
		select {
		case r := <-d.io.keyChan:
			if r == 's' {
				outputBoard(p, d, world, turns)
			}
			if r == 'p' {
				fmt.Printf("Execution Paused at turn %d\n", turns)
				for x := true; x == true; {
					select {
					case pauseInput := <-d.io.keyChan:
						if pauseInput == 's' {
							outputBoard(p, d, world, turns)
						}
						if pauseInput == 'p' {
							x = false
							fmt.Println("Continuing")
						}
						if pauseInput == 'q' {
							p.turns = turns
							x = false
						}
					}
				}
			}
			if r == 'q' {
				p.turns = turns
			}

		default:

		}
		//Keyboard input section end

	}

	finalAlive := countAlive(p, world)
	outputBoard(p, d, world, p.turns)

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
