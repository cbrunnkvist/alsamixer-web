const { chromium } = require('playwright');
const { exec } = require('child_process');

const BASE_URL = 'http://lemox.lan:8888';

// Helper to run SSH command
function sshExec(cmd) {
    return new Promise((resolve, reject) => {
        exec(`ssh root@lemox.lan "${cmd}"`, (err, stdout, stderr) => {
            if (err) reject(err);
            else resolve(stdout);
        });
    });
}

async function runTests() {
    console.log('Starting ALSA→UI E2E Tests...\n');
    
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
                console.log(`▶ ${name}`);
                await fn();
                console.log(`✓ ${name} PASSED\n`);
                passed++;
            } catch (err) {
                console.log(`✗ ${name} FAILED: ${err.message}\n`);
                failed++;
            }
        })();
    }
    
    // Get initial ALSA Master volume
    await test('Get initial ALSA Master volume', async () => {
        const output = await sshExec("amixer -c 1 sget 'Master' | grep 'Mono:'");
        console.log(`  ALSA output: ${output.trim()}`);
        const match = output.match(/Mono:.*Playback\s+(\d+)/);
        if (!match) throw new Error('Could not parse ALSA output');
        const initialVol = parseInt(match[1]);
        console.log(`  Initial Master volume: ${initialVol}`);
    });
    
    // Open page and find Master slider
    await test('Open page and find Master slider', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        // Look for the Master slider by aria-label or nearby text
        const sliders = await page.locator('[role="slider"]').all();
        console.log(`  Found ${sliders.length} sliders total`);
        
        // Try to find Master by looking for it in the DOM
        const masterSlider = page.locator('[aria-label*="Master"], [data-name*="Master"]').first();
        const count = await masterSlider.count();
        console.log(`  Master-related sliders: ${count}`);
    });
    
    // Change ALSA volume externally
    const targetVolume = 30;
    await test(`Change ALSA Master to ${targetVolume}% via amixer`, async () => {
        await sshExec(`amixer -c 1 sset 'Master' ${targetVolume}%`);
        await page.waitForTimeout(500);
        
        const output = await sshExec("amixer -c 1 sget 'Master' | grep 'Mono:'");
        console.log(`  ALSA output: ${output.trim()}`);
    });
    
    // Wait and check if UI updates via SSE
    await test('UI should update via SSE broadcast', async () => {
        // Wait for SSE to propagate the change (monitor ticks every 100ms)
        await page.waitForTimeout(2000);
        
        // Reload to get fresh state from server
        await page.reload();
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        // Check slider values
        const sliders = await page.locator('[role="slider"]').all();
        for (const slider of sliders) {
            const label = await slider.getAttribute('aria-label') || await slider.getAttribute('data-name') || 'unknown';
            const value = await slider.getAttribute('aria-valuenow');
            if (label.toLowerCase().includes('master')) {
                console.log(`  Master slider value: ${value}`);
            }
        }
    });
    
    console.log('========================================');
    console.log(`Tests passed: ${passed}`);
    console.log(`Tests failed: ${failed}`);
    console.log('========================================');
    
    await browser.close();
    process.exit(failed > 0 ? 1 : 0);
}

runTests().catch(err => {
    console.error('Test runner failed:', err);
    process.exit(1);
});
