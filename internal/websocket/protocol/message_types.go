package protocol

// ClientMessageType is the message from client to server
type ClientMessageType string

const (
	ClientCreateGame       ClientMessageType = "game.create"
	ClientJoinGame         ClientMessageType = "game.join"
	ClientEnterMatchmaking ClientMessageType = "matchmaking.enter"
	ClientMakeMove         ClientMessageType = "game.move"
	ClientResign           ClientMessageType = "game.resign"
	ClientOfferDraw        ClientMessageType = "draw.offer"
	ClientAcceptDraw       ClientMessageType = "draw.accept"
	ClientDeclineDraw      ClientMessageType = "draw.decline"
)

// ServerMessageType is the message from server to clients
type ServerMessageType string

const (
	ServerConnectionReady ServerMessageType = "connection.ready"
	ServerGameCreated     ServerMessageType = "game.created"
	ServerGameJoined      ServerMessageType = "game.joined"
	ServerGameState       ServerMessageType = "game.state"
	ServerDrawOffered     ServerMessageType = "draw.offered"
	ServerGameOver        ServerMessageType = "game.over"
	ServerError           ServerMessageType = "error"
)
