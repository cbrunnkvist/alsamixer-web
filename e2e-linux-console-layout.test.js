const { chromium } = require('playwright');

const BASE_URL = process.env.E2E_BASE_URL;
if (!BASE_URL) {
    console.error('Error: E2E_BASE_URL environment variable is required');
    process.exit(1);
}

async function runTests() {
    console.log('Starting Linux Console Layout E2E tests...');
    
    const browser = await chromium.launch({
        executablePath: '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser',
        headless: false,
        args: ['--no-sandbox']
    });
    
    const context = await browser.newContext();
    const page = await context.newPage();
    
    let passed = 0;
    let failed = 0;
    
    async function test(name, fn) {
        console.log(`\n▶ ${name}`);
        try {
            await fn();
            console.log(`✓ ${name} PASSED`);
            passed++;
        } catch (err) {
            console.log(`✗ ${name} FAILED: ${err.message}`);
            failed++;
        }
    }

    async function checkNavButtons(width) {
        console.log(`  Testing at width: ${width}px`);
        await page.setViewportSize({ width, height: 800 });
        await page.goto(`${BASE_URL}/?theme=linux-console`, { waitUntil: 'domcontentloaded' });
        
        // Wait for controls to load
        await page.waitForSelector('.mixer-control', { timeout: 10000 });
        
        // Give it a moment to initialize
        await page.waitForTimeout(1000);

        const isCompact = await page.evaluate(() => document.querySelector('.mixer-card').classList.contains('is-compact'));
        console.log(`  Is compact mode active: ${isCompact}`);

        const prevBtn = page.locator('[data-nav="prev"]');
        const nextBtn = page.locator('[data-nav="next"]');

        if (width >= 960 && !isCompact) {
            // At 1000px, none of the nav buttons should be displayed
            const prevVisible = await prevBtn.isVisible();
            const nextVisible = await nextBtn.isVisible();
            if (prevVisible || nextVisible) {
                throw new Error(`Nav buttons should be hidden at ${width}px (Prev: ${prevVisible}, Next: ${nextVisible})`);
            }
            console.log(`  Verified: Nav buttons hidden at ${width}px`);
        } else {
            // At 500px and 860px, compact mode should be active
            if (!isCompact) {
                throw new Error(`Compact mode should be active at ${width}px`);
            }

            // At first: left nav button should not be displayed
            if (await prevBtn.isVisible()) {
                throw new Error('Previous button should be visible at the first control');
            }
            console.log('  Verified: Previous button hidden at first control');

            // Repeatedly click the right button all the way to the right
            let clicks = 0;
            while (await nextBtn.isVisible() && clicks < 20) {
                await nextBtn.click();
                await page.waitForTimeout(500); // Wait for scroll/animation
                clicks++;
            }
            console.log(`  Clicked Next ${clicks} times to reach the end`);

            // At the end: right button should be the one hidden
            if (await nextBtn.isVisible()) {
                throw new Error('Next button should be hidden at the last control');
            }
            console.log('  Verified: Next button hidden at last control');
        }
    }

    await test('Breakpoint 500px', () => checkNavButtons(500));
    await test('Breakpoint 860px', () => checkNavButtons(860));
    await test('Breakpoint 1000px', () => checkNavButtons(1000));

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
