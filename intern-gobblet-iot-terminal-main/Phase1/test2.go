package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Gobblet represents a game piece with a size (small, medium, large) and an owner (Player 1 or Player 2)
type Gobblet struct {
	Size  int // 1 = Small, 2 = Medium, 3 = Large
	Owner int // 1 = Player 1, 2 = Player 2
}

// Stack represents a stack of Gobblets in a cell
type Stack []Gobblet

// The Board is a 3x3 grid where each cell contains a stack of Gobblets
type Board [3][3]Stack

// Initialize the board and player turn
var board Board
var playerTurn = 1

// Track how many pieces of each size each player has placed
var pieceCount = map[int]map[int]int{
	1: {1: 0, 2: 0, 3: 0}, // Player 1's pieces
	2: {1: 0, 2: 0, 3: 0}, // Player 2's pieces
}

// Function to clear the screen (so only the updated board is visible)
func clearScreen() {
	cmd := exec.Command("clear") // Default for Linux/macOS
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls") // Windows version
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// Function to print the board
func printBoard() {
	// clearScreen() // Clears the screen before printing the updated board
	fmt.Println("\nCurrent Board:")
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if len(board[i][j]) == 0 {
				fmt.Print("  .   ") // Empty cell
			} else {
				top := board[i][j][len(board[i][j])-1]      // Get the top Gobblet in the stack
				fmt.Printf(" %d%d   ", top.Owner, top.Size) // Print owner and size
			}
		}
		fmt.Println() // Move to the next row
	}
	fmt.Println()

	// Print the debug board
	printDebugState()
}

// Function to print the full debug board (shows all pieces in each stack)
func printDebugState() {
	fmt.Println("\nDebug Board (All Stacks):")

	const cellWidth = 25 // Fixed width for each cell to ensure alignment

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			// Format cell position
			cellContent := fmt.Sprintf("[%d,%d]: ", i, j)

			// Add Gobblet information
			if len(board[i][j]) == 0 {
				cellContent += "(empty)"
			} else {
				for _, gobblet := range board[i][j] {
					cellContent += fmt.Sprintf("(%d,%d) ", gobblet.Owner, gobblet.Size)
				}
			}

			// Adjust spacing to ensure all cells are aligned
			fmt.Printf("%-*s", cellWidth, cellContent)

			// Print separator between columns, except for the last column
			if j < 2 {
				fmt.Print("| ")
			}
		}
		fmt.Println() // Move to the next row
	}
	fmt.Println()
}

// Function to check if a player has won
func checkWin() int {
	// Check rows and columns
	for i := 0; i < 3; i++ {
		if winner := checkLine(board[i][0], board[i][1], board[i][2]); winner != 0 {
			return winner
		}
		if winner := checkLine(board[0][i], board[1][i], board[2][i]); winner != 0 {
			return winner
		}
	}
	// Check diagonals
	if winner := checkLine(board[0][0], board[1][1], board[2][2]); winner != 0 {
		return winner
	}
	if winner := checkLine(board[0][2], board[1][1], board[2][0]); winner != 0 {
		return winner
	}
	return 0
}

// Helper function to check if three stacks in a row, column, or diagonal belong to the same player
func checkLine(a, b, c Stack) int {
	if len(a) > 0 && len(b) > 0 && len(c) > 0 {
		if a[len(a)-1].Owner == b[len(b)-1].Owner && b[len(b)-1].Owner == c[len(c)-1].Owner {
			return a[len(a)-1].Owner
		}
	}
	return 0
}

// Function to place a new Gobblet on the board
func placePiece(row, col, size int) bool {
	if row < 0 || row >= 3 || col < 0 || col >= 3 {
		fmt.Println("Invalid position!")
		return false
	}
	if pieceCount[playerTurn][size] >= 3 {
		fmt.Println("You have already used all three pieces of this size!")
		return false
	}
	if len(board[row][col]) > 0 && board[row][col][len(board[row][col])-1].Size >= size {
		fmt.Println("Cannot place smaller Gobblet over a larger one!")
		return false
	}
	board[row][col] = append(board[row][col], Gobblet{Size: size, Owner: playerTurn})
	pieceCount[playerTurn][size]++
	return true
}

// Function to move an existing Gobblet from one position to another
func movePiece(fromRow, fromCol, toRow, toCol int) bool {
	if fromRow < 0 || fromRow >= 3 || fromCol < 0 || fromCol >= 3 || toRow < 0 || toRow >= 3 || toCol < 0 || toCol >= 3 {
		fmt.Println("Invalid move!")
		return false
	}
	if len(board[fromRow][fromCol]) == 0 {
		fmt.Println("No Gobblet to move!")
		return false
	}
	top := board[fromRow][fromCol][len(board[fromRow][fromCol])-1]
	if top.Owner != playerTurn {
		fmt.Println("You can only move your own Gobblet!")
		return false
	}
	if len(board[toRow][toCol]) > 0 && board[toRow][toCol][len(board[toRow][toCol])-1].Size >= top.Size {
		fmt.Println("Cannot place a Gobblet over a larger one!")
		return false
	}
	board[fromRow][fromCol] = board[fromRow][fromCol][:len(board[fromRow][fromCol])-1] // Remove from original position
	board[toRow][toCol] = append(board[toRow][toCol], top)                             // Place in new position
	return true
}

// Main game loop
func main() {
	fmt.Println("Welcome to Gobblet Game!")

	for {
		printBoard() // Show the board before each turn

		fmt.Printf("Player %d, choose action: (1) PLACE = '1 x y 3 (Size)', (2) MOVE = '2 x1 y1 x2 y2': ", playerTurn)
		var action, row, col, size, toRow, toCol int

		_, err := fmt.Scan(&action)
		if err != nil {
			fmt.Println("Invalid input, please try again.")
			continue
		}

		// Placing a new Gobblet
		if action == 1 {
			_, err = fmt.Scan(&row, &col, &size)
			if err != nil || !placePiece(row, col, size) {
				continue
			}
		} else if action == 2 { // Moving an existing Gobblet
			_, err = fmt.Scan(&row, &col, &toRow, &toCol)
			if err != nil || !movePiece(row, col, toRow, toCol) {
				continue
			}
		} else {
			fmt.Println("Invalid action! Use 1 to place, 2 to move.")
			continue
		}

		if winner := checkWin(); winner != 0 {
			printBoard()
			fmt.Printf("Player %d wins!\n", winner)
			os.Exit(0)
		}

		playerTurn = 3 - playerTurn
	}
}
