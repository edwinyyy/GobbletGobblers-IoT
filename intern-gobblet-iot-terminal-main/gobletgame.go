package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"goblets/config"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Gobblet struct {
	Size  int
	Owner int
}

type Stack []Gobblet
type Board [3][3]Stack

type GameState struct {
	Board      Board
	PlayerTurn int
	Winner     int // ✅ New field to track winner
}

var (
	board      Board
	playerTurn = 1
	gameID     string
	playerID   int
	mqttClient mqtt.Client
	mu         sync.Mutex
)

func clearScreen() {
	cmd := exec.Command("clear")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func printBoard() {
	// clearScreen()
	fmt.Println("\nCurrent Board:")
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if len(board[i][j]) == 0 {
				fmt.Print("  .   ")
			} else {
				top := board[i][j][len(board[i][j])-1]
				fmt.Printf(" %d%d   ", top.Owner, top.Size)
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

func setupMQTT() {
	certpool := x509.NewCertPool()
	pemCerts, err := ioutil.ReadFile("root-CA.pem")
	if err != nil {
		log.Fatal("Error loading Root CA:", err)
	}
	certpool.AppendCertsFromPEM(pemCerts)

	cert, err := tls.LoadX509KeyPair("device.pem.crt", "private.pem.key")
	if err != nil {
		log.Fatal("Error loading certificates:", err)
	}

	opts := mqtt.NewClientOptions().
		AddBroker(config.Conf.BrokerURL).
		SetClientID(fmt.Sprintf("GobbletPlayer-%d", time.Now().UnixNano())).
		SetTLSConfig(&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      certpool,
		}).
		SetKeepAlive(30 * time.Second). // ✅ Ensure connection stays active
		SetPingTimeout(20 * time.Second).
		SetAutoReconnect(true) // ✅ Reconnect if disconnected

	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("❌ MQTT Connection Error:", token.Error())
	}

	topic := "gobblet/game/" + gameID
	fmt.Println("✅ Connected to AWS IoT Core! Subscribing to:", topic)

	// ✅ Use QoS 1 for reliable message delivery
	if token := mqttClient.Subscribe(topic, 1, onMessageReceived); token.Wait() && token.Error() != nil {
		log.Fatal("❌ Subscription Error:", token.Error())
	}
	fmt.Println("✅ Subscribed to topic:", topic)
}

func loadGameState() bool {
	topic := "gobblet/game/" + gameID

	stateChan := make(chan GameState, 1) // ✅ Channel to receive the first valid game state

	// ✅ Subscribe to retained message
	token := mqttClient.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		var state GameState
		err := json.Unmarshal(msg.Payload(), &state)
		if err != nil {
			fmt.Println("❌ Error decoding game state from IoT Core:", err)
			return
		}

		// ✅ Load the game state
		select {
		case stateChan <- state:
		default:
		}
	})

	token.Wait()
	if token.Error() != nil {
		fmt.Println("❌ Error subscribing to game state:", token.Error())
		return false
	}

	// ✅ Wait for the first message or timeout after 2 seconds
	select {
	case state := <-stateChan:
		board = state.Board
		playerTurn = state.PlayerTurn
		fmt.Println("✅ Game state loaded from AWS IoT Core retained message!")

		// ✅ Immediately print the board
		printBoard()

		// ✅ If a winner exists, display it on all terminals
		if state.Winner != 0 {
			fmt.Printf("🎉 Player %d wins!\n", state.Winner)
		}

		return true
	case <-time.After(2 * time.Second): // Timeout to avoid infinite waiting
		fmt.Println("⚠ No retained game session found in IoT Core. Creating a new session.")
		return false
	}
}

func saveGameState() {
	winner := checkWin()
	state := GameState{Board: board, PlayerTurn: playerTurn, Winner: winner}

	data, _ := json.Marshal(state)
	topic := "gobblet/game/" + gameID

	fmt.Println("📤 Sending game state to AWS IoT Core:", string(data))

	// ✅ Retain message and ensure Player 2 receives the latest state
	token := mqttClient.Publish(topic, 1, true, data)
	token.Wait()

	if winner != 0 {
		fmt.Printf("🎉 Player %d wins!\n", winner)
	}
}

func publishMove() {
	mu.Lock()
	winner := checkWin()
	state := GameState{Board: board, PlayerTurn: playerTurn, Winner: winner}
	mu.Unlock()

	data, _ := json.Marshal(state)
	topic := "gobblet/game/" + gameID

	fmt.Println("📤 Sending move to AWS IoT Core:", string(data))

	// ✅ Ensure message is retained so opponent sees the latest move
	token := mqttClient.Publish(topic, 1, true, data)
	token.Wait()

	// ✅ Immediately print the board for both players
	printBoard()

	// ✅ If there is a winner, show the message
	if winner != 0 {
		fmt.Printf("🎉 Player %d wins!\n", winner)
		time.Sleep(3 * time.Second) // Allow time for Player 2 to receive update
		return
	}
}

func onMessageReceived(client mqtt.Client, msg mqtt.Message) {
	mu.Lock()
	defer mu.Unlock()

	fmt.Println("📥 Received move from AWS IoT Core:", string(msg.Payload()))

	var state GameState
	err := json.Unmarshal(msg.Payload(), &state)
	if err != nil {
		fmt.Println("❌ Error decoding state:", err)
		return
	}

	// ✅ Ensure board updates properly
	board = state.Board
	playerTurn = state.PlayerTurn

	printBoard() // ✅ Force print board immediately for both players

	// ✅ If there's a winner, show it
	if state.Winner != 0 {
		fmt.Printf("🎉 Player %d wins!\n", state.Winner)
		os.Exit(0) // Ensure game stops when there's a winner
	} else {
		fmt.Println("✅ Board updated from AWS IoT Core!")
	}
}

func placePiece(row, col, size int) bool {
	if size < 1 || size > 3 {
		fmt.Println("❌ Invalid move: Goblet size must be between 1 and 3!")
		return false
	}

	if row < 0 || row >= 3 || col < 0 || col >= 3 {
		fmt.Println("❌ Invalid move: Out of bounds!")
		return false
	}

	if len(board[row][col]) > 0 && board[row][col][len(board[row][col])-1].Size >= size {
		fmt.Println("❌ Invalid move: Cannot place a smaller piece on a larger one!")
		return false
	}

	// ✅ Place the goblet before checking for a win
	board[row][col] = append(board[row][col], Gobblet{Size: size, Owner: playerTurn})

	// ✅ Save game state and publish move
	saveGameState()
	publishMove()

	// ✅ If a winner is detected, print the message and return
	winner := checkWin()
	if winner != 0 {
		fmt.Printf("🎉 Player %d wins!\n", winner)
		return true
	}

	// ✅ Switch turn after move and publish immediately
	playerTurn = 3 - playerTurn
	publishMove()

	return true
}

func movePiece(fromRow, fromCol, toRow, toCol int) bool {
	if fromRow < 0 || fromRow >= 3 || fromCol < 0 || fromCol >= 3 || toRow < 0 || toRow >= 3 || toCol < 0 || toCol >= 3 {
		fmt.Println("❌ Invalid move: Out of bounds!")
		return false
	}
	if len(board[fromRow][fromCol]) == 0 {
		fmt.Println("❌ Invalid move: No piece to move!")
		return false
	}
	top := board[fromRow][fromCol][len(board[fromRow][fromCol])-1]
	if top.Owner != playerTurn {
		fmt.Println("❌ Invalid move: You can only move your own pieces!")
		return false
	}
	if len(board[toRow][toCol]) > 0 && board[toRow][toCol][len(board[toRow][toCol])-1].Size >= top.Size {
		fmt.Println("❌ Invalid move: Cannot place a smaller piece on a larger one!")
		return false
	}

	// ✅ Move the piece
	board[fromRow][fromCol] = board[fromRow][fromCol][:len(board[fromRow][fromCol])-1]
	board[toRow][toCol] = append(board[toRow][toCol], top)

	// ✅ Save game state and publish move
	saveGameState()
	publishMove()

	// ✅ If a winner is detected, print the message and return
	winner := checkWin()
	if winner != 0 {
		fmt.Printf("🎉 Player %d wins!\n", winner)
		return true
	}

	// ✅ Switch turn after move and publish immediately
	playerTurn = 3 - playerTurn
	publishMove()

	return true
}

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

func checkLine(a, b, c Stack) int {
	if len(a) > 0 && len(b) > 0 && len(c) > 0 {
		if a[len(a)-1].Owner == b[len(b)-1].Owner && b[len(b)-1].Owner == c[len(c)-1].Owner {
			return a[len(a)-1].Owner
		}
	}
	return 0
}

func main() {
	fmt.Print("Enter a 5-digit Game ID: ")
	fmt.Scan(&gameID)

	if len(gameID) != 5 {
		fmt.Println("❌ Invalid Game ID! Must be 5 digits.")
		os.Exit(1)
	}

	setupMQTT()

	fmt.Println("🔍 Checking for existing game session...")
	if !loadGameState() {
		fmt.Println("🆕 No game found. Creating new game session.")
		saveGameState()
	}

	fmt.Print("Enter Player Number (1 , 2) or (3 for Spectating): ")
	fmt.Scan(&playerID)

	// ✅ Player 2 continuously checks for updates
	// ✅ Player 2 continuously checks for updates
	go func() {
		for {
			time.Sleep(2 * time.Second)
			// Force MQTT client to re-subscribe if needed

		}
	}()

	for {
		printBoard()

		// ✅ Spectator Mode: Keep watching the game
		if playerID == 3 {
			fmt.Print("\r👀 You are now Spectating the Game")
			continue
		}

		// ✅ Player should see "Waiting for opponent's move..." only ONCE
		if playerTurn != playerID {
			fmt.Print("\nWaiting for opponent's move...") // ✅ Print only once
			for playerTurn != playerID {
				time.Sleep(1 * time.Second) // ✅ Keep checking silently
			}
			fmt.Println() // ✅ Move to a new line after waiting
		}

		// ✅ Check if the game has ended before making a move
		if winner := checkWin(); winner != 0 {
			printBoard()
			fmt.Printf("🎉 Player %d wins!\n", winner)
			os.Exit(0)
		}

		fmt.Printf("Player %d, choose action: (1) PLACE = '1 x y size', (2) MOVE = '2 x1 y1 x2 y2': ", playerTurn)
		var action, row, col, size, toRow, toCol int

		_, err := fmt.Scan(&action)
		if err != nil {
			fmt.Println("Invalid input, please try again.")
			time.Sleep(2 * time.Second)
			continue
		}

		if action == 1 {
			_, err = fmt.Scan(&row, &col, &size)
			if err != nil {
				fmt.Println("❌ Invalid input for place action. Try again.")
				time.Sleep(2 * time.Second)
				continue
			}

			if !placePiece(row, col, size) {
				fmt.Println("❌ Invalid placement. Try again.")
				time.Sleep(2 * time.Second)
				continue
			}
		} else if action == 2 {
			_, err = fmt.Scan(&row, &col, &toRow, &toCol)
			if err != nil {
				fmt.Println("❌ Invalid input for move action. Try again.")
				time.Sleep(2 * time.Second)
				continue
			}

			if !movePiece(row, col, toRow, toCol) {
				fmt.Println("❌ Invalid move. Try again.")
				time.Sleep(2 * time.Second)
				continue
			}
		} else {
			fmt.Println("❌ Invalid action! Use 1 to place, 2 to move.")
			time.Sleep(2 * time.Second)
			continue
		}
	}
}
