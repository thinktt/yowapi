const fs = require('fs')
const crypto = require('crypto')


process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0'
const token = process.env.YOW_TOKEN 
console.log(token)

const whitePlayers = ["Capablanca", "Wizard", "Fischer"] 
// const blackPlayers = ["Tex", "Ben", "Stanley"]
const blackPlayers = ["Lacey"] 


const cmps = JSON.parse(fs.readFileSync('./dist/personalities.json', 'utf8'))

const cmpNames = Object.keys(cmps)



async function startGame(whitePlayerId, blackPlayerId) {
  try {
    const response = await fetch('https://localhost:8443/games2', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}` // Adding the Authorization header
      },
      body: JSON.stringify({
        whitePlayer: { id: whitePlayerId, type: "cmp" },
        blackPlayer: { id: blackPlayerId, type: "cmp" }
      }),
    })

    const gameData = await response.json()
        
    return gameData

  } catch (error) {
    console.error('Error starting game:', error)
    return
  }
 
}

async function startGames() {
  while (true) {
    // const whitePlayer = getRandomPlayer(whitePlayers)
    // const blackPlayer = getRandomPlayer(blackPlayers)
    // const whitePlayer = selectRandomCMP(whitePlayers)
    // const blackPlayer = selectRandomCMP(blackPlayers)
    const whitePlayer = "Willow"
    const blackPlayer = "Willow"

    const game = await startGame(whitePlayer, blackPlayer)
    if (!game) {
      console.error(`unalbe to start game ${whitePlayer} vs ${blackPlayer}`)
      break
    }
    console.log(`\x1b[31mstarted game ${game.whitePlayer.id} vs ${game.blackPlayer.id}\x1b[0m`)
    
    const result = await waitForGameEnd(game.id)
    console.log()
    if (result) {
      console.error(result)
      break
    }
  }
}

async function waitForGameEnd(gameID) {
  const controller = new AbortController()
  const signal = controller.signal

  try {
    const response = await fetch(`https://localhost:8443/streams/${gameID}`, { signal })
    const reader = response.body.getReader()
    const decoder = new TextDecoder()

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      const eventStr = decoder.decode(value)
      const event = parseSSE(eventStr)
      const data = JSON.parse(event.data) 

      const move = data.moves.split(' ').pop()
      process.stdout.write(`${move} `)
      
      if (data.winner === "draw") {
        console.log("the bots drew each other")
        process.exit(0)
      }

      if (data.winner) {
        console.log("closing stream ", data.id)
        controller.abort() 

        return null
      }
    }
  } catch (error) {
    return error
  }
}


// .................helpers......................
function selectRandomCMP() {
  // completly unesseary crypto random, just to make sure it's really random!
  const randomIndex = crypto.randomInt(0, cmpNames.length)
  const cmpName = cmpNames.splice(randomIndex, 1)[0]
  return cmpName 
}

function getRandomPlayer(players) {
  return players[Math.floor(Math.random() * players.length)]
}

function parseSSE(eventString) {
  const result = {}
  eventString.trim().split('\n').forEach(line => {
    const [key, value] = line.split(/:(.+)/)
    result[key.trim()] = value.trim()
  })
  return result
}

startGames()