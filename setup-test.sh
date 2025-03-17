#!/bin/bash

wd=$PWD
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

    cd "$wd" || exit
done

cd "$wd" || exit

mkdir -p "test_data/project_that_has_future_commits"
cd "test_data/project_that_has_future_commits" || exit
git init
git config user.name "tester"
git config user.email "tester@test.com"
echo "some content" > "text.txt"
git add "text.txt"
commit_date=$(date -d "+1 year" --iso-8601=seconds)
git commit --date="$commit_date" -m "commit message"

cd "$wd" || exit

mkdir -p "test_data/project_by_another_contributor"
cd "test_data/project_by_another_contributor" || exit
git init
git config user.name "another_tester"
git config user.email "another_tester@test.com"
echo "Content added by another contributor" > "another_file.txt"
git add "another_file.txt"
git commit -m "Commit by another contributor"
