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

func workerController(p golParams, d distributorChans, worldRef [][]byte, workerChans []chan rune, turnInfo chan int) {
	//Keyboard input section
	//r: input signal from rune channel
	//t: 2 seconds time signal from bool channel
controlLoop:
	for {

		select {
		case r := <-d.io.keyChan:
			if r == 's' {
				for i := range workerChans {
					workerChans[i] <- 's'
				}
				turns := <-turnInfo
				outputBoard(p, d, worldRef, turns)
			}
			if r == 'p' {
				turns := <-turnInfo
				fmt.Printf("Execution Paused at turn %d\n", turns)
				for x := true; x == true; {
					select {
					case pauseInput := <-d.io.keyChan:
						if pauseInput == 's' {
							outputBoard(p, d, worldRef, turns)
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
				for i := range workerChans {
					workerChans[i] <- 'q'
				}
				break controlLoop
				//p.turns = turns
			}
		case t := <-d.io.timeChan:
			if t {
				fmt.Println(len(countAlive(p, worldRef)))

			}
		default:

		}
	}

	//Keyboard input section end
}

func sendWorldToWorkers(p golParams, world [][]byte, golWorkerChans []chan byte, golCumulativeThreadHeights []int) {
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
}

func removeHaloAndMergeThreads(p golParams, golResultChans []chan [][]byte, golHalos [][][]byte, golNonHalos [][][]byte, golThreadHeights []int, golCumulativeThreadHeights []int) [][]byte {
	newWorld := allocateSlice(p.imageHeight, p.imageWidth)

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
	return newWorld
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

/*
func haloExchange(p golParams, thread [][]byte, in <-chan byte, out chan<- byte) [][]byte{
	temp := make([]byte, p.imageWidth)
	for

}

*/

func golWorker2A(p golParams, cellChan <-chan byte, out chan<- [][]byte, heightInfo int, workers []worker, workerNumber int, controlChan chan rune, turnInfo chan int) {
	height := heightInfo + 2
	width := p.imageWidth
	thisWorker := workers[workerNumber]
	aboveWorker := workers[(workerNumber-1+p.threads)%p.threads]
	belowWorker := workers[(workerNumber+1)%p.threads]
	turn := 0

	//Makes thread with incoming cells
	threadWorld := allocateSlice(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := <-cellChan
			threadWorld[y][x] = cell
		}
	}

turnLoop:
	for ; turn < p.turns; turn++ {

		for x := 0; x < p.imageWidth; x++ {
			thisWorker.upper <- threadWorld[1][x]
			thisWorker.lower <- threadWorld[height-2][x]
			threadWorld[height-1][x] = <-belowWorker.upper
			threadWorld[0][x] = <-aboveWorker.lower
		}
		threadWorld = golLogic(threadWorld)
		select {
		case r := <-controlChan:
			if r == 's' {
				if workerNumber == 0 {
					turnInfo <- turn
				}
				break turnLoop
			}
		}
	}

	out <- threadWorld
}

//aaa
func golWorker2B(p golParams, cellChan <-chan byte, out chan<- [][]byte, heightInfo int, workers []worker, workerNumber int, controlChan chan rune) {
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
turnLoop:
	for turn := 0; turn < p.turns; turn++ {
		for x := 0; x < p.imageWidth; x++ {
			threadWorld[height-1][x] = <-belowWorker.upper
			threadWorld[0][x] = <-aboveWorker.lower
			thisWorker.upper <- threadWorld[1][x]
			thisWorker.lower <- threadWorld[height-2][x]
		}
		threadWorld = golLogic(threadWorld)
		select {
		case r := <-controlChan:
			if r == 's' {
				break turnLoop
			}
		}
	}

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
	var worldRef = world
	turnInfo := make(chan int)

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
	//fmt.Println("world init done")

	if p.turns != 0 {

		//Calculating thread height
		golThreadHeights := calculateThreadHeight(p)
		for index := 0; index < p.threads; index++ {
			if golThreadHeights[index] == 0 {
				p.threads = index
				break
			}
		}
		golCumulativeThreadHeights := make([]int, p.threads)
		golCumulativeThreadHeights[0] = golThreadHeights[0]
		for i := 1; i < len(golCumulativeThreadHeights); i++ {
			golCumulativeThreadHeights[i] = golCumulativeThreadHeights[i-1] + golThreadHeights[i]
		}
		//fmt.Println("thread height calculated")

		//Initialise halo channels
		aboveHaloExchanges := make([]chan byte, p.threads)
		belowHaloExchanges := make([]chan byte, p.threads)
		for i := 0; i < p.threads; i++ {
			aboveHaloExchanges[i] = make(chan byte)
			belowHaloExchanges[i] = make(chan byte)
		}
		//fmt.Println("signals init")
		//fmt.Println("halo exchange init")
		workers := make([]worker, p.threads)
		for i := 0; i < p.threads; i++ {
			//w(i).upperGet = w(i - 1).lowerSend
			//w(i).lowerGet = w(i + 1).upperSend
			workers[i] = worker{
				upper: aboveHaloExchanges[i],
				lower: belowHaloExchanges[i]}
		}
		//fmt.Println("workers init")

		//Init slices of channels and slices of threads
		//Slice of threads after gol logic with halo
		golHalos := make([][][]byte, p.threads)
		//Slice of threads after removing halo
		golNonHalos := make([][][]byte, p.threads)
		//Slice of channel of workers before gol logic
		golWorkerChans := make([]chan byte, p.threads)
		//Slice of channels for control signals
		golWorkerSignals := make([]chan rune, p.threads)
		//Slice of channel of workers after gol logic
		golResultChans := make([]chan [][]byte, p.threads)
		for i := range golResultChans {
			golResultChans[i] = make(chan [][]byte, golThreadHeights[i]+2)
		}
		for i := range golWorkerChans {
			golWorkerChans[i] = make(chan byte)
		}
		for i := range golWorkerSignals {
			golWorkerSignals[i] = make(chan rune)
		}
		//fmt.Println("workerchan resultchan init")

		//Go routine starts here
		for i := range golWorkerChans {
			if i%2 == 0 {
				go golWorker2A(p, golWorkerChans[i], golResultChans[i], golThreadHeights[i], workers, i, golWorkerSignals[i], turnInfo)
			} else {
				go golWorker2B(p, golWorkerChans[i], golResultChans[i], golThreadHeights[i], workers, i, golWorkerSignals[i])
			}
		}
		go workerController(p, d, worldRef, golWorkerSignals, turnInfo)

		sendWorldToWorkers(p, world, golWorkerChans, golCumulativeThreadHeights)

		world = removeHaloAndMergeThreads(p, golResultChans, golHalos, golNonHalos, golThreadHeights, golCumulativeThreadHeights)

		//Updating the world with new world
		/*
			for y := 0; y < p.imageHeight; y++ {
				for x := 0; x < p.imageWidth; x++ {
					world[y][x] = newWorld[y][x]
				}
			}

		*/

	}

	finalAlive := countAlive(p, world)
	outputBoard(p, d, worldRef, p.turns)

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
