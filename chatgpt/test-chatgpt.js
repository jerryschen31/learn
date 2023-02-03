// Command+Shift+i to open ChatGPT in a new window

// write an example of Promises in Javascript


// Example of a Promise in JavaScript
let myPromise = new Promise((resolve, reject) => {
    let condition = true;
  
    if (condition) {
      resolve('Promise was fulfilled');
    } else {
      reject('Promise was rejected');
    }
  });
  
  myPromise.then(response => console.log(response)).catch(error => console.log(error));