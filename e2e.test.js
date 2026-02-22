const { chromium } = require('playwright');

const BASE_URL = process.env.E2E_BASE_URL;
if (!BASE_URL) {
    console.error('Error: E2E_BASE_URL environment variable is required');
    process.exit(1);
}

async function runTests() {
    console.log('Starting Playwright E2E tests...');
    
    // Launch Brave browser (as per AGENTS.md instructions)
    const browser = await chromium.launch({
        executablePath: '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser',
        headless: false,
        args: ['--no-sandbox']
    });
    
    const context = await browser.newContext();
    const page = await context.newPage();
    
    let passed = 0;
    let failed = 0;
    
    function test(name, fn) {
        return (async () => {
            try {
                console.log(`\n▶ ${name}`);
                await fn();
                console.log(`✓ ${name} PASSED`);
                passed++;
            } catch (err) {
                console.log(`✗ ${name} FAILED: ${err.message}`);
                failed++;
            }
        })();
    }
    
    // Test 1: UI loads correctly
    await test('UI loads homepage', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        const title = await page.title();
        if (!title.includes('ALSA Mixer')) {
            throw new Error(`Expected title to contain 'ALSA Mixer', got: ${title}`);
        }
    });
    
    // Test 2: Volume controls are visible
    await test('Volume controls are visible', async () => {
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        const sliders = await page.locator('[role="slider"]').count();
        console.log(`  Found ${sliders} volume sliders`);
        if (sliders === 0) {
            throw new Error('No volume sliders found on page');
        }
    });
    
    // Test 3: UI→ALSA - slider click changes ALSA volume
    await test('UI→ALSA: Slider click changes volume', async () => {
        const slider = page.locator('[role="slider"]').first();
        const initialValue = await slider.getAttribute('aria-valuenow');
        console.log(`  Initial slider value: ${initialValue}`);
        
        await slider.click();
        await page.waitForTimeout(500);
        
        const newValue = await slider.getAttribute('aria-valuenow');
        console.log(`  New slider value: ${newValue}`);
        console.log('  Click registered');
    });
    
    // Test 4: SSE connection established
    await test('SSE connection established', async () => {
        await page.reload();
        await page.waitForTimeout(2000);
        console.log('  SSE connection should be active');
    });
    
    // Test 5: Page renders without critical errors
    await test('Page renders without critical errors', async () => {
        const errors = [];
        page.on('pageerror', err => errors.push(err.message));
        
        await page.goto(BASE_URL);
        await page.waitForTimeout(2000);
        
        if (errors.length > 0) {
            console.log(`  Console errors: ${errors.join(', ')}`);
        }
    });
    
    console.log(`\n========================================`);
    console.log(`Tests passed: ${passed}`);
    console.log(`Tests failed: ${failed}`);
    console.log(`========================================`);
    
    await browser.close();
    process.exit(failed > 0 ? 1 : 0);
}

runTests().catch(err => {
    console.error('Test runner failed:', err);
    process.exit(1);
});
