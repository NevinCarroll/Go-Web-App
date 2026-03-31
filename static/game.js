// game.js - Castle Defender Game Logic

class Vec2 {
    constructor(x, y) {
        this.x = x;
        this.y = y;
    }

    sub(v) {
        return new Vec2(this.x - v.x, this.y - v.y);
    }

    add(v) {
        return new Vec2(this.x + v.x, this.y + v.y);
    }

    len() {
        return Math.sqrt(this.x * this.x + this.y * this.y);
    }

    unit() {
        const len = this.len();
        return len > 0 ? new Vec2(this.x / len, this.y / len) : new Vec2(0, 0);
    }

    scaled(s) {
        return new Vec2(this.x * s, this.y * s);
    }

    dot(v) {
        return this.x * v.x + this.y * v.y;
    }
}

class SeededRNG {
    constructor(seed) {
        // simple LCG constants
        this.state = seed % 2147483647;
        if (this.state <= 0) this.state += 2147483646;
    }

    next() {
        // return float [0,1)
        this.state = (this.state * 16807) % 2147483647;
        return (this.state - 1) / 2147483646;
    }
}

const EnemyType = {
    Default: 0,
    Fast: 1,
    Tank: 2
};

const TowerType = {
    Standard: 1,
    Rapid: 2,
    Sniper: 3
};

const EnemyConfigs = {
    [EnemyType.Default]: { health: 1.0, speed: 60.0, color: '#dc143c' },
    [EnemyType.Fast]: { health: 0.5, speed: 90.0, color: '#dda0dd' },
    [EnemyType.Tank]: { health: 3.0, speed: 40.0, color: '#8b0000' }
};

const TowerConfigs = {
    [TowerType.Standard]: { radius: 120, damage: 1.0, attackCadence: 0.16, cost: 100, color: '#ffd700' },
    [TowerType.Rapid]: { radius: 90, damage: 0.5, attackCadence: 0.08, cost: 100, color: '#00ff7f' },
    [TowerType.Sniper]: { radius: 180, damage: 2.5, attackCadence: 0.35, cost: 100, color: '#4169e1' }
};

class Enemy {
    constructor(pos, typeID) {
        const cfg = EnemyConfigs[typeID];
        this.pos = pos;
        this.health = cfg.health;
        this.maxHealth = cfg.health;
        this.speed = cfg.speed;
        this.typeID = typeID;
        this.pathIndex = 0;
        this.color = cfg.color;
    }

    update(dt, path) {
        if (this.pathIndex >= path.length - 1) return;
        const target = path[this.pathIndex + 1];
        const dir = target.sub(this.pos).unit();
        this.pos = this.pos.add(dir.scaled(this.speed * dt));
        if (this.pos.sub(target).len() < 5) {
            this.pathIndex++;
        }
    }

    reachedEnd(path) {
        return this.pathIndex >= path.length - 1;
    }

    isDead() {
        return this.health <= 0;
    }

    takeDamage(damage) {
        this.health -= damage;
    }
}

class Tower {
    constructor(pos, typeID) {
        const cfg = TowerConfigs[typeID];
        this.pos = pos;
        this.radius = cfg.radius;
        this.damage = cfg.damage;
        this.attackCadence = cfg.attackCadence;
        this.cooldown = 0;
        this.typeID = typeID;
        this.color = cfg.color;
    }

    update(dt, enemies) {
        this.cooldown -= dt;
        if (this.cooldown > 0) return null;

        let target = null;
        let closest = 1e9;
        for (const e of enemies) {
            if (e.isDead()) continue;
            const d = e.pos.sub(this.pos).len();
            if (d <= this.radius && d < closest) {
                target = e;
                closest = d;
            }
        }

        if (target) {
            target.takeDamage(this.damage);
            this.cooldown = this.attackCadence;
            return target;
        }
        return null;
    }
}

class Laser {
    constructor(start, end) {
        this.start = start;
        this.end = end;
        this.time = 0.15;
    }
}

class Game {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.path = [
            new Vec2(80, 160), new Vec2(240, 160), new Vec2(240, 420), new Vec2(560, 420),
            new Vec2(560, 220), new Vec2(840, 220), new Vec2(840, 640), new Vec2(940, 640)
        ];
        this.enemies = [];
        this.towers = [];
        this.lasers = [];
        this.lives = 5;
        this.gold = 300;
        this.wave = 0;
        this.selectedTowerType = TowerType.Standard;
        this.placePreview = false;
        this.previewPos = new Vec2(0, 0);
        this.lastTime = 0;
        this.statsDiv = document.getElementById('stats');
        this.goldEarned = 0;
        this.enemiesKilled = 0;
        this.towersPlaced = 0;

        const user = document.body.dataset.user || 'guest';
        this.saveKey = `castle_save_${user}`;

        this.seed = Math.floor(Math.random() * 1000000);
        this.rng = new SeededRNG(this.seed);
        this.waveInProgress = false;

        const params = new URLSearchParams(window.location.search);
        const shouldContinue = params.get('continue') === '1';
        if (shouldContinue) {
            this.loadSavedGame().then((loaded) => {
                if (loaded) {
                    this.waveInProgress = false;
                    this.updateWaveButtonState();
                }
            });
        }

        this.setupInput();
        this.setupWaveControls();
    }

    setupInput() {
        this.canvas.addEventListener('mousemove', (e) => {
            if (!this.placePreview) return;
            const rect = this.canvas.getBoundingClientRect();
            this.previewPos = new Vec2(e.clientX - rect.left, e.clientY - rect.top);
        });

        this.canvas.addEventListener('mousedown', (e) => {
            const rect = this.canvas.getBoundingClientRect();
            const pos = new Vec2(e.clientX - rect.left, e.clientY - rect.top);
            if (e.button === 0) { // left click
                this.handleLeftClick(pos);
            } else if (e.button === 2) { // right click
                this.placePreview = false;
            }
        });

        this.canvas.addEventListener('contextmenu', (e) => e.preventDefault());

        document.addEventListener('keydown', (e) => {
            if (e.key === '1') this.startTowerPlacement(TowerType.Standard);
            if (e.key === '2') this.startTowerPlacement(TowerType.Rapid);
            if (e.key === '3') this.startTowerPlacement(TowerType.Sniper);
            if (e.key === 'Escape') {
                if (this.modalVisible) {
                    this.hideQuitModal();
                } else {
                    this.showQuitModal();
                }
                e.preventDefault();
            }
        });

        const btnStandard = document.getElementById('btnStandard');
        const btnRapid = document.getElementById('btnRapid');
        const btnSniper = document.getElementById('btnSniper');

        if (btnStandard) btnStandard.addEventListener('click', () => this.startTowerPlacement(TowerType.Standard));
        if (btnRapid) btnRapid.addEventListener('click', () => this.startTowerPlacement(TowerType.Rapid));
        if (btnSniper) btnSniper.addEventListener('click', () => this.startTowerPlacement(TowerType.Sniper));
    }

    setupWaveControls() {
        this.nextWaveButton = document.getElementById('btnNextWave');
        if (this.nextWaveButton) {
            this.nextWaveButton.addEventListener('click', () => {
                if (this.waveInProgress) return;
                if (this.enemies.length > 0) return;
                this.wave++;
                this.waveInProgress = true;
                this.spawnEnemyWave();
                this.updateWaveButtonState();
            });
        }

        this.waveInfoDiv = document.getElementById('waveInfo');
        this.updateWaveButtonState();
    }

    updateWaveButtonState() {
        if (this.nextWaveButton) {
            this.nextWaveButton.disabled = this.waveInProgress;
            this.nextWaveButton.innerText = this.waveInProgress ? 'Wave in Progress' : 'Spawn Next Wave';
        }
        if (this.waveInfoDiv) {
            this.waveInfoDiv.innerText = `Wave ${this.wave + 1} ready (seed: ${this.seed})`;
        }
    }

    async saveGame() {
        const saveData = {
            wave: this.wave,
            lives: this.lives,
            gold: this.gold,
            towers: this.towers.map(t => ({ x: t.pos.x, y: t.pos.y, typeID: t.typeID })),
            seed: this.seed
        };
        try {
            await fetch('/save', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ...saveData, towers: JSON.stringify(saveData.towers) })
            });
        } catch (err) {
            console.error('Could not save game to server', err);
        }
    }

    async loadSavedGame() {
        try {
            const resp = await fetch('/load');
            if (!resp.ok) return false;
            const data = await resp.json();
            this.wave = Number(data.wave) || 0;
            this.lives = Number(data.lives) || 5;
            this.gold = Number(data.gold) || 300;
            const towers = JSON.parse(data.towers || '[]');
            this.towers = towers.map(t => new Tower(new Vec2(Number(t.x), Number(t.y)), Number(t.typeID)));
            this.enemies = [];
            this.seed = Number(data.seed) || this.seed;
            this.rng = new SeededRNG(this.seed);
            return true;
        } catch (err) {
            console.error('Failed to load saved game', err);
            return false;
        }
    }

    clearSave() {
        fetch('/delete-save', { method: 'POST' }).catch(err => console.error('Failed to clear server save', err));
    }

    startTowerPlacement(typeID) {
        const saved = localStorage.getItem(this.saveKey);
        if (!saved) return false;
        try {
            const data = JSON.parse(saved);
            this.wave = Number(data.wave) || 0;
            this.lives = Number(data.lives) || 5;
            this.gold = Number(data.gold) || 300;
            this.path = this.path || this.path;
            this.towers = (data.towers || []).map(t => new Tower(new Vec2(Number(t.x), Number(t.y)), Number(t.typeID)));
            this.enemies = [];
            this.seed = Number(data.seed) || this.seed;
            this.rng = new SeededRNG(this.seed);
            return true;
        } catch (err) {
            console.error('Failed to load saved game', err);
            return false;
        }
    }

    clearSave() {
        localStorage.removeItem(this.saveKey);
    }

    startTowerPlacement(typeID) {
        this.selectedTowerType = typeID;
        this.placePreview = true;
    }

    handleLeftClick(pos) {
        if (!this.placePreview) {
            return;
        }

        const cfg = TowerConfigs[this.selectedTowerType];
        if (this.gold >= cfg.cost && this.isValidPlacement(pos)) {
            this.towers.push(new Tower(pos, this.selectedTowerType));
            this.gold -= cfg.cost;
            this.towersPlaced++;
            this.placePreview = false;
        } else {
            // Keep preview active; invalid placement just does not place
            // (player can adjust position and click again)
        }
    }

    isValidPlacement(pos) {
        if (this.inPathArea(pos)) return false;
        if (this.isTowerOverlap(pos)) return false;
        return true;
    }

    inPathArea(pos) {
        for (let i = 0; i < this.path.length - 1; i++) {
            if (this.distancePointToSegment(pos, this.path[i], this.path[i + 1]) < 30) return true;
        }
        return false;
    }

    distancePointToSegment(p, segA, segB) {
        const segmentVector = segB.sub(segA);
        const pointVector = p.sub(segA);
        const squaredLen = segmentVector.dot(segmentVector);
        if (squaredLen === 0) return p.sub(segA).len();
        let t = pointVector.dot(segmentVector) / squaredLen;
        t = Math.max(0, Math.min(1, t));
        const closest = segA.add(segmentVector.scaled(t));
        return p.sub(closest).len();
    }

    isTowerOverlap(pos) {
        for (const t of this.towers) {
            if (pos.sub(t.pos).len() < 24) return true;
        }
        return false;
    }

    spawnEnemyWave() {
        const enemiesThisWave = 2 + Math.floor(this.wave / 5);
        for (let i = 0; i < enemiesThisWave; i++) {
            const r = this.rng.next();
            let typeID = EnemyType.Default;
            if (r < 0.25) typeID = EnemyType.Tank;
            else if (r < 0.6) typeID = EnemyType.Fast;
            this.enemies.push(new Enemy(this.path[0], typeID));
        }
    }

    update(dt) {
        if (!this.waveInProgress) {
            // awaiting player to press next wave
        }

        for (let i = this.enemies.length - 1; i >= 0; i--) {
            const e = this.enemies[i];
            e.update(dt, this.path);
            if (e.reachedEnd(this.path)) {
                this.lives--;
                this.enemies.splice(i, 1);
            } else if (e.isDead()) {
                this.gold += 25;
                this.goldEarned += 25;
                this.enemiesKilled++;
                this.enemies.splice(i, 1);
            }
        }

        if (this.waveInProgress && this.enemies.length === 0) {
            this.waveInProgress = false;
            this.updateWaveButtonState();
            this.saveGame();
        }

        for (const t of this.towers) {
            const shot = t.update(dt, this.enemies);
            if (shot) {
                this.lasers.push(new Laser(t.pos, shot.pos));
            }
        }

        for (let i = this.lasers.length - 1; i >= 0; i--) {
            const l = this.lasers[i];
            l.time -= dt;
            if (l.time <= 0) this.lasers.splice(i, 1);
        }

        if (this.lives <= 0) {
            this.gameOver('All lives lost');
        }
    }

    render() {
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        // Draw path
        this.ctx.strokeStyle = 'yellow';
        this.ctx.lineWidth = 4;
        this.ctx.beginPath();
        this.ctx.moveTo(this.path[0].x, this.path[0].y);
        for (let i = 1; i < this.path.length; i++) {
            this.ctx.lineTo(this.path[i].x, this.path[i].y);
        }
        this.ctx.stroke();

        // Draw path points
        this.ctx.fillStyle = 'white';
        for (const p of this.path) {
            this.ctx.beginPath();
            this.ctx.arc(p.x, p.y, 4, 0, 2 * Math.PI);
            this.ctx.fill();
        }

        // Draw preview
        if (this.placePreview) {
            const cfg = TowerConfigs[this.selectedTowerType];
            this.ctx.strokeStyle = this.isValidPlacement(this.previewPos) ? 'blue' : 'red';
            this.ctx.lineWidth = 1;
            this.ctx.beginPath();
            this.ctx.arc(this.previewPos.x, this.previewPos.y, cfg.radius, 0, 2 * Math.PI);
            this.ctx.stroke();
            this.ctx.beginPath();
            this.ctx.arc(this.previewPos.x, this.previewPos.y, 10, 0, 2 * Math.PI);
            this.ctx.fill();
        }

        // Draw towers
        for (const t of this.towers) {
            this.ctx.fillStyle = t.color;
            this.ctx.beginPath();
            this.ctx.arc(t.pos.x, t.pos.y, 10, 0, 2 * Math.PI);
            this.ctx.fill();
            this.ctx.fillStyle = 'white';
            this.ctx.beginPath();
            this.ctx.arc(t.pos.x, t.pos.y, 2, 0, 2 * Math.PI);
            this.ctx.fill();
        }

        // Draw enemies
        for (const e of this.enemies) {
            this.ctx.fillStyle = e.color;
            this.ctx.beginPath();
            this.ctx.arc(e.pos.x, e.pos.y, 7, 0, 2 * Math.PI);
            this.ctx.fill();
            this.ctx.strokeStyle = e.color;
            this.ctx.lineWidth = 3;
            this.ctx.beginPath();
            this.ctx.moveTo(e.pos.x, e.pos.y);
            this.ctx.lineTo(e.pos.x + 10, e.pos.y);
            this.ctx.stroke();
        }

        // Draw lasers
        this.ctx.strokeStyle = 'orange';
        this.ctx.lineWidth = 2;
        for (const l of this.lasers) {
            this.ctx.beginPath();
            this.ctx.moveTo(l.start.x, l.start.y);
            this.ctx.lineTo(l.end.x, l.end.y);
            this.ctx.stroke();
        }

        // Update stats
        const waveStatus = this.waveInProgress ? 'In Progress' : 'Waiting';
        this.statsDiv.innerHTML = `Lives: ${this.lives} Gold: ${this.gold} Wave: ${this.wave} (${waveStatus}) Towers: ${this.towers.length} Enemies: ${this.enemies.length} Selected: ${this.towerTypeName(this.selectedTowerType)}`;
    }

    towerTypeName(t) {
        switch (t) {
            case TowerType.Standard: return 'Standard';
            case TowerType.Rapid: return 'Rapid';
            case TowerType.Sniper: return 'Sniper';
            default: return 'None';
        }
    }

    gameOver(reason) {
        this.clearSave();
        localStorage.setItem('gameOverReason', reason);
        localStorage.setItem('goldEarned', this.goldEarned);
        localStorage.setItem('enemiesKilled', this.enemiesKilled);
        localStorage.setItem('towersPlaced', this.towersPlaced);
        localStorage.setItem('wavesSurvived', this.wave);
        window.location.href = '/game-over';
    }

    showQuitModal() {
        this.modalVisible = true;
        const modal = document.getElementById('quitModal');
        if (!modal) return;
        modal.classList.remove('hidden');

        const saveBtn = document.getElementById('quitSaveBtn');
        const noSaveBtn = document.getElementById('quitNoSaveBtn');
        const cancelBtn = document.getElementById('quitCancelBtn');

        if (saveBtn) {
            saveBtn.onclick = async () => {
                await this.saveGame();
                this.hideQuitModal();
                window.location.href = '/';
            };
        }

        if (noSaveBtn) {
            noSaveBtn.onclick = () => {
                this.hideQuitModal();
                this.gameOver('Quit without saving');
            };
        }

        if (cancelBtn) {
            cancelBtn.onclick = () => {
                this.hideQuitModal();
            };
        }
    }

    hideQuitModal() {
        this.modalVisible = false;
        const modal = document.getElementById('quitModal');
        if (modal) modal.classList.add('hidden');
    }

    quitGame() {
        this.showQuitModal();
    }

    loop(timestamp) {
        const dt = (timestamp - this.lastTime) / 1000;
        this.lastTime = timestamp;
        this.update(dt);
        this.render();
        requestAnimationFrame((ts) => this.loop(ts));
    }

    start() {
        requestAnimationFrame((ts) => this.loop(ts));
    }
}

const canvas = document.getElementById('gameCanvas');
const game = new Game(canvas);
game.start();