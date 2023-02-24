// HTML elements
let divSelectRoom = document.getElementById('selectRoom');
let divConsultingRoom = document.getElementById('consultingRoom');
let inputRoomNumber = document.getElementById('roomNumber');
let btnGoRoom = document.getElementById('goRoom');
let localVideo = document.getElementById('localVideo');
let remoteVideo = document.getElementById('remoteVideo');

let roomNumber, localStream, remoteStream, rtcPeerConnection, isCaller;

// STUN servers used by client browser for identifying this client public IP address+port(s), and if it's accessible for streaming
const iceServers = {
    'iceServer': [
        {'urls': 'stun:stun.services.mozilla.com'},
        {'urls': 'stun:stun.l.google.com:19302'}
    ]
}

const streamConstraints = {
    audio: true,
    video: true,
}

// create a connection to the signaling server (the URL typed into the browser)
const socket = io();

// when client clicks on the "Join Room" button, client browser emits a 'create or join' message back to the signaling server
btnGoRoom.onclick = () => {
    console.log('in onClick()');
    if(inputRoomNumber.value === ''){
        alert('please type a room name');
    } else {
        roomNumber = inputRoomNumber.value;
        socket.emit('create or join', roomNumber);
        divSelectRoom.style = 'display: none';
        divConsultingRoom.style = 'display: block';
    }
}

// once the first client browser connects to the signaling server, the client receives back a "created" signal
socket.on('created', room => {
    console.log('created room', room);
    // since room has been created and this socket is connected to that room, client browser can now open up a video stream
    navigator.mediaDevices.getUserMedia(streamConstraints)
        .then(stream => {
            localStream = stream;
            localVideo.srcObject = stream;
            isCaller = true;
        })
        .catch(err => {
            console.log('an error occurred', err);
        })
});

// once the second+ client browser connects to the signaling server, the client receives a "joined" signal
socket.on('joined', room => {
    console.log('joined room', room);
    // since this socket is connected to the room, client browser can now open up a video stream and let the signaling server its ready
    navigator.mediaDevices.getUserMedia(streamConstraints)
        .then(stream => {
            localStream = stream;
            localVideo.srcObject = stream;
            isCaller = false;
            socket.emit('ready', room);            
        })
        .catch(err => {
            console.log('an error occurred', err);
        })
});

// additional user has joined the call - all joined clients receive a ready message
socket.on('ready', () => {
    if(isCaller){
        // first client (caller) creates a new empty video for the additional user and sends an Offer to connect to the second client
        // [TODO] expand this to work for 3+ clients (this currently only works for 2 clients)
        addVideoElement( divConsultingRoom.childElementCount );
        sendOffer(socket, localStream, iceServers, roomNumber);
    }
})

// clients who are not the caller receive an offer
socket.on('offer', (event) => {
    if(!isCaller){
        console.log('received offer', event);
        // add new video element
        addVideoElement( divConsultingRoom.childElementCount );
        handleOffer(socket, event, localStream, iceServers);
    }
})

socket.on('answer', event => {
    console.log('received answer', event);
    handleAnswer(event);
})

// add ICE candidate (IP address or TURN server IP) to RTC peer connection
socket.on('candidate', event => {
    const candidate = new RTCIceCandidate({
        sdpMLineIndex: event.label,
        candidate: event.candidate
    })
    console.log('received candidate', candidate);
    rtcPeerConnection.addIceCandidate(candidate);
})

// add a new video element to the HTML div containing all video elements
function addVideoElement( numParticipants ) {
    newVideo = document.createElement('video');
    newVideo.setAttribute('id', 'remoteVideo');
    newVideo.autoplay = true;
    newVideo.controls = true;
    newVideo.height = 300;
    newVideo.width = 300;
    divConsultingRoom.appendChild(newVideo);
}

// add remote stream we have received as a new video element to our webpage
function addStream(event) {
    let remoteVideo = document.getElementById('remoteVideo');
    console.log('adding remote stream video', event);
    remoteVideo.srcObject = event.streams[0];
    remoteStream = event.streams[0];
    console.log('remote video object added', remoteVideo.srcObject);
    return false;
}

// callers finds an ICE candidate (valid IP address or TURN server) for the client who just joined the call
async function oniceCandidate(event){
    if(event.candidate){
        console.log('sending ice candidate', event.candidate);
        socket.emit('candidate', {
            type: 'candidate',
            label: event.candidate.sdpMLineIndex,
            id: event.candidate.sdpMid,
            candidate: event.candidate.candidate,
            room: roomNumber,
        })
    }
}

// RTC peer connection object contains info on this client (public IP, stream info,...) that will be sent to the other client(s)
async function createPeerConnection(localStream, iceServers){
    // gets public IP of this machine -> need this for setting up P2P connection with a remote peer
    rtcPeerConnection = new RTCPeerConnection(iceServers);
    rtcPeerConnection.oniceCandidate = oniceCandidate;
    // sets up event handler for appropriately handling audio/video streams
    rtcPeerConnection.ontrack = addStream;
    // video track - add to local stream
    rtcPeerConnection.addTrack(localStream.getTracks()[0], localStream);
    // audio track - add to local stream 
    rtcPeerConnection.addTrack(localStream.getTracks()[1], localStream);
    console.log(rtcPeerConnection);
}


// first client (caller) creates an offer and sends it to the second+ client
async function sendOffer(socket, localStream, iceServers, roomNumber){
    // create everything on this (caller) end for establishing a P2P connection - get public IP, video stream,...
    await createPeerConnection(localStream, iceServers);
    // create and send offer -> returns Promise with session description (sdp)        
    rtcPeerConnection.createOffer()
        .then(sessionDescription => {
            console.log('sending offer', sessionDescription);
            rtcPeerConnection.setLocalDescription(sessionDescription);
            socket.emit('offer', {
                type: 'offer',
                sdp: sessionDescription,
                room: roomNumber,                
            })
        })
        .catch( err=> {
            console.log(err);
        })    
}

// remote partcipant handles offer and sends answer
async function handleOffer(socket, event, localStream, iceServers){
    await createPeerConnection(localStream, iceServers);
    // set remote sdp
    await rtcPeerConnection.setRemoteDescription(new RTCSessionDescription(event));
    // create and send answer -> returns Promise with session description (sdp)
    rtcPeerConnection.createAnswer()
        .then(sessionDescription => {    
            console.log('sending answer', sessionDescription);
            rtcPeerConnection.setLocalDescription(sessionDescription);
            socket.emit('answer', {
                type: 'answer',
                sdp: sessionDescription,
                room: roomNumber,                
            })
        })
        .catch( err=> {
            console.log(err);
        })    
}

async function handleAnswer( event ){
    await rtcPeerConnection.setRemoteDescription(new RTCSessionDescription(event));
}