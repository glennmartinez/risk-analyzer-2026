# ue_scraper.py
# A simple scraper for Unreal Engine Automation Test Framework docs
# Install libs: pip install requests beautifulsoup4 lxml playwright
# Run: python ue_scraper.py
# Saves to 'ue_automation_docs' folder as Markdown files for RAG (easy to chunk/index)

import os
import time
from urllib.parse import urljoin

from bs4 import BeautifulSoup
from lxml import html
from playwright.sync_api import sync_playwright

BASE_URL = "https://dev.epicgames.com/documentation/en-us/unreal-engine/automation-test-framework-in-unreal-engine"
OUTPUT_DIR = "ue_automation_docs"
os.makedirs(OUTPUT_DIR, exist_ok=True)


def scrape_page(page, url):
    page.goto(url, wait_until="load", timeout=60000)
    time.sleep(3)  # Allow dynamic content to render
    html = page.content()

    # Check for bot detection
    if "Just a moment" in html or "security check" in html.lower():
        print(f"Blocked by bot detection on: {url}")
        return None

    soup = BeautifulSoup(html, "lxml")

    # Extract title
    title = soup.find("h1").text.strip() if soup.find("h1") else "Untitled"

    # Extract main content (assuming <article> or similar for UE docs)
    content_div = soup.find("article") or soup.find("div", {"class": "content"})
    if content_div:
        # Remove nav/scripts/etc
        for elem in content_div.find_all(["script", "nav", "footer"]):
            elem.extract()
        content = content_div.get_text(separator="\n", strip=True)
    else:
        content = soup.get_text(separator="\n", strip=True)

    # Save as MD
    slug = url.split("/")[-1] or "index"
    md_path = os.path.join(OUTPUT_DIR, f"{slug}.md")
    with open(md_path, "w", encoding="utf-8") as f:
        f.write(f"# {title}\n\n")
        f.write(f"URL: {url}\n\n")
        f.write(content)

    print(f"Saved: {md_path}")

    return soup, title


def extract_sub_links(page, base_url):
    print(f"Page title: {page.title()}")
    print(f"Sidebar count: {page.locator('edc-sidebar').count()}")
    print(f"Table of contents count: {page.locator('table-of-contents').count()}")

    # Use Playwright locator with XPath to find the section li
    xpath = "/html/body/app-root/div[2]/site-nav/edc-sidebar/div/table-of-contents/div[3]/nav/ul/li/ul/li[19]"
    locator = page.locator(f"xpath={xpath}")
    print(f"XPath locator count: {locator.count()}")

    if not locator.count():
        print("Sidebar section not found via XPath")
        # Try broader XPath or alternative
        alt_locator = page.locator("edc-sidebar table-of-contents nav a")
        alt_count = alt_locator.count()
        print(f"Alternative locator count: {alt_count}")
        if alt_count > 0:
            print("Using alternative locator")
            link_elements = alt_locator
        else:
            return []
    else:
        # Find all a elements within this li subtree that have href
        link_elements = locator.locator("a[href]")

    links = []
    for i in range(link_elements.count()):
        href = link_elements.nth(i).get_attribute("href")
        if href:
            link_url = urljoin(base_url, href)
            links.append(link_url)

    return links


if __name__ == "__main__":
    with sync_playwright() as p:
        browser = p.firefox.launch(headless=True)

        # Scrape main page with a new page
        page = browser.new_page()
        page.set_viewport_size({"width": 1280, "height": 720})
        main_soup, _ = scrape_page(page, BASE_URL)
        page.close()

        # Extract sub-links from sidebar - need to re-scrape main page for links
        page = browser.new_page()
        page.set_viewport_size({"width": 1280, "height": 720})
        main_soup, _ = scrape_page(page, BASE_URL)
        sub_links = extract_sub_links(page, BASE_URL)
        print(f"Found {len(sub_links)} sub-links")
        page.close()

        # Scrape each sub-page with a new page each time
        for link in sub_links:
            page = browser.new_page()
            page.set_viewport_size({"width": 1280, "height": 720})
            result = scrape_page(page, link)
            page.close()
            if result is None:
                continue  # Skip blocked pages
            time.sleep(5)  # Polite delay

        browser.close()

    print("Scraping complete. Files saved to", OUTPUT_DIR)
