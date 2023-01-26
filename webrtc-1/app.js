const express = require('express');
const app = express();
let http = require('http').Server(app);

const port = process.env.PORT || 3000;

// specify a server that will use socket.io
let io = require('socket.io')(http)

//static hosting of public folder
app.use(express.static('public'));

http.listen(port, () => {
    console.log('listening on ', port);
});

// signalling server
io.on('connection', socket => {
    console.log('a user connected');

    // listener
    socket.on('create or join', room => {
        console.log('create or join to room', room);
        console.log('io.sockets keys', io.sockets.sockets.keys());

        const myRoom = io.sockets.adapter.sids.size;
        // const roomId = io.sockets.sockets.keys().next().value;
        // const myRoom = io.sockets.adapter.rooms.get(roomId) || {length: 0};
        // console.log('room id', roomId);
        console.log('io.sockets.adapter', io.sockets.adapter);
        console.log('myRoom:', myRoom);
        const numClients = myRoom; // .size;
        console.log(room, 'has', numClients, 'clients');

        if(numClients <= 1){
            socket.join(room);
            socket.emit('created', room);
        } else if (numClients > 1 && numClients <= 4) {
            socket.join(room);
            socket.emit('joined', room);
        } else {
            socket.emit('full', room);
        }
    })

    socket.on('ready', room => {
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