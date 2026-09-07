package main

import (
	"fmt"
)

type Identity interface {
	GetName() string
	SetName( string )
}

type User struct {
	name string
}

func (u *User) GetName() string {
	return u.name
}

func (u *User) SetName( new_name string ) {
	u.name = new_name
}
// by defining GetName() and SetName( string ), the POINTER to a User struct (*User) satisfies the Identity interface

func (u *User) GetDoubleName() string {
	return u.name + " " + u.name
}

// constructor for User struct - creates a new user, returns a pointer to this new user struct
func NewUser( new_user_name string ) *User {
	return &User{ name: new_user_name }
}

// describe returns the underlying value(s) and type within the underlying type of the interface (in this case, the specific instance of the User struct)
func describe(i Identity) {
	fmt.Printf("(%v, %T)\n", i, i)
}

func main() {
	me_as_user := NewUser( "Jerry" ) // this declares a variable of type *User that satsifies Identity interface
	fmt.Println( me_as_user.GetName() )
	fmt.Println( me_as_user.name )  // this works because the variable is a pointer to the underlying User struct (Go auto-references to the actual struct so *me_as_user is not needed)
	fmt.Println( me_as_user.GetDoubleName() )  // this also works because me_as_user exposes the actual User struct

	var me_as_identity Identity = NewUser( "Jerry" ) // this explicitly declares a variable of type Identity (an interface type); could also do `me_as_identity := Identity(NewUser("Jerry"))`
	fmt.Println( me_as_identity.GetName() )
	// fmt.Println( me_as_identity.name )  // this does NOT work (will error that name is undefined since interface doesn't allow fields) because the type here is Identity, not the User struct
	// fmt.Println( me_as_identity.GetDoubleName() ) // this also does not work since Identity interface does not expose GetDoubleName()
	me_as_identity.SetName( "Evelyn" )
	fmt.Println( me_as_identity.GetName() )

	describe( me_as_identity )
}

// so interfaces are a way to hide the internal fields and other (non-interface) methods from an end-user

// Note: interfaces that have not yet been declared a value (e.g., var me_as_identity Identity) are nil by default

