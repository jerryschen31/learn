let divSelectRoom = document.getElementById('selectRoom');
let divConsultingRoom = document.getElementById('consultingRoom');
let inputRoomNumber = document.getElementById('roomNumber');
let btnGoRoom = document.getElementById('goRoom');
let localVideo = document.getElementById('localVideo');
let remoteVideo = document.getElementById('remoteVideo');

let roomNumber, localStream, remoteStream, rtcPeerConnection, isCaller;

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

// create a connection to the signalling server (the URL typed into the browser)
const socket = io();

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

socket.on('created', room => {
    console.log('created room', room);
    // getUserMedia returns a promise with the stream
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

socket.on('joined', room => {
    console.log('joined room', room);
    // getUserMedia returns a promise with the stream
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

// additional user has joined the call - client who is the caller receives ready signal
socket.on('ready', () => {
    if(isCaller){
        // add new video element for remote video once offer/answer is done
        addVideoElement( divConsultingRoom.childElementCount );
        sendOffer(socket, localStream, iceServers, roomNumber, rtcPeerConnection);
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
    handleAnswer(event, rtcPeerConnection);
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

// creates a peer connection
async function createPeerConnection(localStream, iceServers, rtcPeerConnection){
    // gets public IP of this machine and sets up P2P connection
    rtcPeerConnection = new RTCPeerConnection(iceServers);
    rtcPeerConnection.oniceCandidate = oniceCandidate;
    rtcPeerConnection.ontrack = addStream;
    // video track - add to local stream
    rtcPeerConnection.addTrack(localStream.getTracks()[0], localStream);
    // audio track - add to local stream 
    rtcPeerConnection.addTrack(localStream.getTracks()[1], localStream);
    return rtcPeerConnection;
}

// remote partcipant handles offer and sends answer
async function handleOffer(socket, event, localStream, iceServers){
    let rtcPeerConnection = await createPeerConnection(localStream, iceServers);
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

// caller creates an offer
async function sendOffer(socket, localStream, iceServers, roomNumber, rtcPeerConnection){
    rtcPeerConnection = await createPeerConnection(localStream, iceServers);
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

async function handleAnswer( event, rtcPeerConnection){
    await rtcPeerConnection.setRemoteDescription(new RTCSessionDescription(event));
}