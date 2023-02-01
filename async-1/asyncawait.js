async function f() {

  let promise = new Promise((resolve, reject) => {
    setTimeout(() => resolve("async/await function done!"), 2000)
  });

  let result = await promise; // wait until the promise resolves (*)

  console.log(result); // "done!"
}

f();
console.log("this runs first...");
