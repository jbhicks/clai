#!/bin/bash
# Download articles for bias analysis

# Function to download article content
download_article() {
    local url="$1"
    local filename="$2"
    curl -s "$url" -o "$filename" 2>/dev/null
    echo "Downloaded: $filename"
}

# Download 3 articles from NY Times headlines
echo "Downloading articles for bias analysis..."
download_article "https://www.nytimes.com/2023/06/15/world/middleeast/syria-russia-assad.html" "article1.html"
download_article "https://www.nytimes.com/2023/06/15/us/politics/trump-oil-tanker-seizures.html" "article2.html"
download_article "https://www.nytimes.com/2023/06/15/arts/television/60-minutes-venezuela-deportations.html" "article3.html"

echo "All articles downloaded."
