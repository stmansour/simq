#!/bin/bash

# Function to display the menu
display_menu() {
    echo "Press"
    echo "d - tree /var/lib/dispatcher"
    echo "s - tree /var/lib/simd"
    echo "r - tree /opt/testsimres"
    echo "x - exit"
    echo -n "Enter your choice: "
}

# Loop to get user input and perform actions
while true; do
    display_menu
    read -n 1 -r choice
    echo  # To move to a new line after the character input

    case "$choice" in
        d)
            exa --tree /var/lib/dispatcher
            ;;
        s)
            exa --tree /var/lib/simd
            ;;
        r)
            exa --tree /opt/testsimres
            ;;
        x | q)
            echo "Exiting..."
            break
            ;;
        *)
            echo "Invalid choice, please try again."
            ;;
    esac
done

