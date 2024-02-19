#!/bin/bash

projects=("project_1" "project_2" "project_3")

for project in "${projects[@]}"; do
    mkdir -p "test_data/$project"
    cd "test_data/$project" || exit
    git init
    git config user.name "tester"
    git config user.email "tester@test.com"

    for ((i = 1; i <= 3; i++)); do
        echo "Content $i" > "file_$i.txt"

        git add "file_$i.txt"

        git commit -m "Commit $i"
    done

    cd ../../ || exit
done
