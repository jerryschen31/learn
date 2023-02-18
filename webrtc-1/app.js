/****************
 This is the signaling server used to establish a connection between two devices.
 Server uses Express as its server framework and Socket.io for relaying messages.
 ****************/

const MAX_CLIENTS = 2

// creates a new instance of the Express web application framework and a NodeJS HTTP Server object. Express is a minimal and flexible Node.js web application framework that provides a robust set of features for web and mobile applications. It is used to create server-side applications and APIs, and can be used with other frameworks such as React, Angular, and Vue.
const express = require('express');
const app = express();
let http = require('http').Server(app);

const port = process.env.PORT || 3000;

// new http server object "io" uses socket.io
let io = require('socket.io')(http)

//static hosting of files in public folder
app.use(express.static('public'));

// call method listen() on the HTTP object to listen for messages on specified port
http.listen(port, () => {
    console.log('listening on ', port);
});

// signaling server - responds to a socket connection on HTTP port (connection request from client browser)
io.on('connection', socket => {
    // signaling server receives a socket connection object
    console.log('a user connected');

    // [TODO] What is a socket?
    // responds to 'create or join' message from client browser
    socket.on('create or join', room => {
        // signaling server receives a room ID from the client (via the socket connection)
        console.log('create or join to room', room);

        // console.log('io.sockets keys', io.sockets.sockets.keys());
        // const roomId = io.sockets.sockets.keys().next().value;
        // const myRoom = io.sockets.adapter.rooms.get(roomId) || {length: 0};
        // console.log('room id', roomId);

        console.log('io.sockets.adapter', io.sockets.adapter);
        const numClients = io.sockets.adapter.sids.size;; // .size;
        console.log(room, 'has', numClients, 'clients');

        // socket connection joins this room
        if(numClients <= 1){
            socket.join(room);
            socket.emit('created', room);
        } else if (numClients > 1 && numClients <= MAX_CLIENTS) {
            socket.join(room);
            socket.emit('joined', room);
        } else {
            socket.emit('full', room);
        }
    })

    // after second+ clients join room, they send 'ready' msg to signaling server
    socket.on('ready', room => {
        // let all joined clients know that the room is ready
        socket.broadcast.to(room).emit('ready');
    })

    socket.on('candidate', event => {
        socket.broadcast.to(event.room).emit('candidate', event);
    })

    socket.on('offer', event => {
        socket.broadcast.to(event.room).emit('offer', event.sdp);
    })

    socket.on('answer', event => {
        socket.broadcast.to(event.room).emit('answer', event.sdp);
    })

})