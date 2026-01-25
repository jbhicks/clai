const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({
    args: [
      '--disable-gpu',
      '--disable-software-rasterizer',
      '--disable-dev-shm-usage'
    ]
  });
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1400, height: 1000 });
  
  // Navigate to models page
  await page.goto('http://localhost:8081/models');
  await page.waitForTimeout(2000);  // Wait for SSE updates
  
  // Scroll to downloads section
  await page.evaluate(() => {
    document.querySelector('h2').scrollIntoView();
  });
  
  // Take screenshot of Active Downloads view
  await page.screenshot({ 
    path: '/home/josh/clai/screenshots/downloads_active.png',
    fullPage: true 
  });
  console.log('Screenshot saved: downloads_active.png');
  
  // Click "Show All"
  const showAllButton = page.locator('button:has-text("Show All")').first();
  if (await showAllButton.isVisible()) {
    await showAllButton.click();
    await page.waitForTimeout(1000);
    
    // Take screenshot of All Downloads view
    await page.screenshot({ 
      path: '/home/josh/clai/screenshots/downloads_all.png',
      fullPage: true 
    });
    console.log('Screenshot saved: downloads_all.png');
  }
  
  await browser.close();
})();
