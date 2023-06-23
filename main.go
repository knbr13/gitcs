package main

func main() {
	email, folder := getInputFromUser()

	if folder != "" {
		scan(folder)
	}

	stats(email)
}
