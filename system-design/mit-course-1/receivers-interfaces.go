
// Python equivalent of an implemented interface
//class EmailSender:
//    def send(self, msg: str) -> None:
//        print(f"Sending email: {msg}")
//
//# Usage:
//sender = EmailSender()
//sender.send("Hello World")

// inteface definition
type Notifier interface {
	Send(msg string) error
}

// inteface type used as a field in a struct
type UserService struct {
	notifier Notifier // Can hold ANY type that has a Send(string) method
}

// concrete implementation of an interface
type EmailSender struct{}

// having a receiver (e EmailSender) converts this func into a method of the EmailSender struct
func (e EmailSender) Send(msg string) error {
	fmt.Println("Sending email:", msg)
	return nil
}

// since the above EmailSender struct now has a Send() method, it satisfies the requirements (i.e., implements) an interface of type Notifier
// this is similar in concept to a subclass extending or deriving from an (abstract) Superclass that just defines abstract methods
// the subclass "overrides" or "implements" the abstract methods in the base class (superclass)

type SMSSender struct{}

func (s SMSSender) Send(msg string) error {
	fmt.Println("Sending SMS:", msg)
	return nil
}

func main() {
	// Both are valid because both implement Notifier
	service1 := UserService{notifier: EmailSender{}}
	service2 := UserService{notifier: SMSSender{}}

	service1.notifier.Send("Hello via Email!")
	service2.notifier.Send("Hello via SMS!")
}

// full Python code equivalent
// from abc import ABC, abstractmethod

// # 1. Interface definition (Base Class)
// class Notifier(ABC):

//     @abstractmethod
//     def send(self, msg: str) -> None:
//         """Any subclass must implement this method."""
//         pass

// # 2. Type used as a field in a class
// class UserService:
//     def __init__(self, notifier: Notifier):
//         # Can hold ANY object that inherits from Notifier
//         self.notifier = notifier

//     def register_user(self, username: str):
//         print(f"User {username} registered.")
//         # Utilizing the interface method
//         self.notifier.send(f"Welcome {username}!")

// # 3. Concrete implementation of the interface
// class EmailSender(Notifier):
//     # The subclass DEFINES (and IMPLEMENTS) the actual method
//     def send(self, msg: str) -> None:
//         print(f"Sending email: {msg}")

// # 4. Another concrete implementation
// class SMSSender(Notifier):
//     def send(self, msg: str) -> None:
//         print(f"Sending SMS: {msg}")

// # --- Execution Example ---
// if __name__ == "__main__":
//     # Initialize the concrete implementations
//     email_tool = EmailSender()
//     sms_tool = SMSSender()

//     # Pass the Email implementation into the service
//     user_service = UserService(email_tool)
//     user_service.register_user("Alice")

