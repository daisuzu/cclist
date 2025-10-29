// Main application
class CCListApp {
    constructor() {
        this.currentPage = null;
        this.repositories = [];
        this.config = null;
        this.sortColumn = 'updated'; // Default sort column
        this.sortDirection = 'desc'; // Default sort direction (newest first)

        this.init();
    }

    async init() {
        // Load initial config
        await this.loadConfig();

        // Setup event listeners
        this.setupEventListeners();

        // Setup routing
        this.setupRouting();

        // Load initial page
        this.navigate(window.location.pathname);
    }

    setupEventListeners() {
        // Discover button
        document.getElementById('discoverBtn').addEventListener('click', () => {
            this.discoverRepositories();
        });

        // Refresh button
        document.getElementById('refreshBtn').addEventListener('click', () => {
            this.navigate(window.location.pathname);
        });

        // Settings button
        document.getElementById('settingsBtn').addEventListener('click', () => {
            this.navigate('/settings');
        });

        // Handle browser back/forward
        window.addEventListener('popstate', (e) => {
            this.navigate(window.location.pathname, false);
        });
    }

    setupRouting() {
        // Intercept all link clicks
        document.addEventListener('click', (e) => {
            if (e.target.tagName === 'A' && e.target.getAttribute('data-link')) {
                e.preventDefault();
                const href = e.target.getAttribute('href');
                this.navigate(href);
            }
        });
    }

    navigate(path, pushState = true) {
        // Cleanup terminal before navigation
        if (this.terminalCleanup) {
            this.terminalCleanup();
            this.terminalCleanup = null;
        }

        // Cleanup shell terminal before navigation
        if (this.shellTerminalCleanup) {
            this.shellTerminalCleanup();
            this.shellTerminalCleanup = null;
        }

        if (pushState) {
            window.history.pushState({}, '', path);
        }

        // Route to appropriate page
        if (path === '/' || path === '') {
            this.showRepositoryList();
        } else if (path === '/settings') {
            this.showSettings();
        } else if (path.startsWith('/repo/')) {
            const repoPath = path.substring(6);
            this.showRepositoryDetail(repoPath);
        } else {
            this.show404();
        }
    }

    async loadConfig() {
        try {
            const response = await fetch('/api/config');
            this.config = await response.json();

            // Update port number in header
            document.getElementById('portNumber').textContent = this.config.port;
        } catch (error) {
            console.error('Failed to load config:', error);
        }
    }

    async loadRepositories() {
        try {
            const response = await fetch('/api/repositories');
            const data = await response.json();
            this.repositories = data.repositories || [];
            return this.repositories;
        } catch (error) {
            console.error('Failed to load repositories:', error);
            return [];
        }
    }

    async showRepositoryList() {
        const main = document.getElementById('mainContent');
        main.innerHTML = '<div class="loading">Loading repositories...</div>';

        const repos = await this.loadRepositories();

        if (repos.length === 0) {
            main.innerHTML = this.renderEmptyState();
            return;
        }

        main.innerHTML = this.renderRepositoryList(repos);
    }

    renderEmptyState() {
        return `
            <div class="empty-state">
                <h2>No repositories registered</h2>
                <p>Get started by discovering repositories or adding them manually.</p>
                <button class="btn" onclick="app.discoverRepositories()">🔍 Discover Repositories</button>
                <button class="btn btn-secondary" onclick="app.navigate('/settings')">⚙️ Go to Settings</button>
            </div>
        `;
    }

    renderRepositoryList(repos) {
        // Sort repositories before rendering
        const sortedRepos = this.sortRepositories([...repos]);

        let html = '<div class="filter-section">';
        html += '<input type="text" id="filterInput" class="filter-input" placeholder="Filter by repository or branch name...">';
        html += '</div>';

        html += '<div class="table-container"><table id="repoTable">';
        html += '<thead><tr>';
        html += this.renderSortableHeader('status', 'Status');
        html += this.renderSortableHeader('repository', 'Repository');
        html += this.renderSortableHeader('branch', 'Branch');
        html += this.renderSortableHeader('session', 'Session');
        html += this.renderSortableHeader('updated', 'Updated');
        html += '<th>Latest Output</th>'; // Not sortable
        html += '<th>Actions</th>'; // Not sortable
        html += '</tr></thead>';
        html += '<tbody>';

        for (const repo of sortedRepos) {
            html += this.renderRepositoryRow(repo);
        }

        html += '</tbody></table></div>';

        // Setup filter and sorting after rendering
        setTimeout(() => {
            this.setupFilter();
            this.setupSorting();
        }, 0);

        return html;
    }

    renderSortableHeader(column, label) {
        const isActive = this.sortColumn === column;
        const direction = isActive ? this.sortDirection : '';
        const arrow = isActive ? (direction === 'asc' ? ' ▲' : ' ▼') : '';
        const activeClass = isActive ? ' active' : '';

        return `<th class="sortable${activeClass}" data-column="${column}">${label}${arrow}</th>`;
    }

    renderRepositoryRow(repo) {
        const hasSession = repo.activeSession !== null;
        const statusIcon = hasSession ? '●' : '📝';
        const statusClass = hasSession ? 'active' : 'history';

        const branch = repo.gitBranch || 'unknown';
        const timeAgo = repo.lastAccessed ? this.formatTimeAgo(new Date(repo.lastAccessed)) : 'N/A';

        let sessionStatus = 'History';
        let sessionClass = 'history';
        if (hasSession) {
            sessionStatus = repo.activeSession.isActive ? 'Active' : 'Idle';
            sessionClass = repo.activeSession.isActive ? 'active' : 'idle';
        }

        const output = hasSession && repo.activeSession.outputPath ?
            'Session available' : 'No recent output';

        let html = '<tr class="repo-row">';
        html += `<td><span class="repo-status ${statusClass}">${statusIcon}</span></td>`;
        html += `<td><a href="/repo/${repo.path}" data-link class="repo-path">${repo.path}</a></td>`;
        html += `<td><span class="branch-name">${branch}</span></td>`;
        html += `<td><span class="session-status ${sessionClass}">${sessionStatus}</span></td>`;
        html += `<td><span class="time-ago">${timeAgo}</span></td>`;
        html += `<td><span class="output-preview">${output}</span></td>`;
        html += `<td><button class="btn btn-sm" onclick="app.showWorktreeModal('${repo.path}'); event.stopPropagation();">+ Worktree</button></td>`;
        html += '</tr>';

        // Add worktree rows if present
        if (repo.worktrees && repo.worktrees.length > 1) {
            for (const wt of repo.worktrees) {
                if (wt.isMain) continue; // Skip main worktree (already shown)

                // Calculate worktree relative path from absolute path
                const wtRelPath = this.getRelativePathFromRoot(wt.path);

                // Check for worktree session
                const wtHasSession = wt.activeSession !== null && wt.activeSession !== undefined;
                const wtStatusIcon = wtHasSession ? '●' : '📝';
                const wtStatusClass = wtHasSession ? 'active' : 'history';

                let wtSessionStatus = 'History';
                let wtSessionClass = 'history';
                if (wtHasSession) {
                    wtSessionStatus = wt.activeSession.isActive ? 'Active' : 'Idle';
                    wtSessionClass = wt.activeSession.isActive ? 'active' : 'idle';
                }

                const wtOutput = wtHasSession && wt.activeSession.outputPath ?
                    'Session available' : 'No session';

                html += '<tr class="repo-row worktree-row">';
                html += `<td><span class="repo-status ${wtStatusClass}">${wtStatusIcon}</span></td>`;
                html += `<td class="worktree-indent"><a href="/repo/${wtRelPath}" data-link class="repo-path">└─ ${wtRelPath}</a></td>`;
                html += `<td><span class="branch-name">${wt.branch}</span></td>`;
                html += `<td><span class="session-status ${wtSessionClass}">${wtSessionStatus}</span></td>`;
                html += `<td><span class="time-ago">N/A</span></td>`;
                html += `<td><span class="output-preview">${wtOutput}</span></td>`;
                html += `<td></td>`; // Empty actions cell for worktree rows
                html += '</tr>';
            }
        }

        return html;
    }

    formatTimeAgo(date) {
        const now = new Date();
        const seconds = Math.floor((now - date) / 1000);

        if (seconds < 60) return 'just now';
        if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
        if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
        return Math.floor(seconds / 86400) + 'd ago';
    }

    async discoverRepositories() {
        const main = document.getElementById('mainContent');
        main.innerHTML = '<div class="loading">Discovering repositories...</div>';

        try {
            const response = await fetch('/api/repositories/discover', {
                method: 'POST'
            });
            const data = await response.json();

            if (data.discovered && data.discovered.length > 0) {
                // Show discovered repositories and allow user to add them
                this.showDiscoveredRepositories(data.discovered);
            } else {
                main.innerHTML = `
                    <div class="empty-state">
                        <h2>No repositories found</h2>
                        <p>No repositories with .claude/ directory were found in the current directory.</p>
                        <button class="btn" onclick="app.navigate('/')">← Back to List</button>
                    </div>
                `;
            }
        } catch (error) {
            console.error('Failed to discover repositories:', error);
            main.innerHTML = `
                <div class="error">
                    Failed to discover repositories: ${error.message}
                </div>
            `;
        }
    }

    showDiscoveredRepositories(discovered) {
        const main = document.getElementById('mainContent');

        let html = '<div class="settings-section">';
        html += '<h2>Discovered Repositories</h2>';
        html += `<p class="text-muted mb-2">Found ${discovered.length} repositories with ClaudeCode history</p>`;

        // Selection controls
        html += '<div style="margin-bottom: 1rem; display: flex; gap: 0.5rem;">';
        html += '<button class="btn btn-sm btn-secondary" onclick="app.selectAllRepositories(true)">✓ Select All</button>';
        html += '<button class="btn btn-sm btn-secondary" onclick="app.selectAllRepositories(false)">✗ Deselect All</button>';
        html += '</div>';

        html += '<div class="table-container"><table>';
        html += '<thead><tr><th style="width: 40px;">Add</th><th>Repository Path</th><th>Full Path</th><th>Status</th></tr></thead>';
        html += '<tbody>';

        for (const repo of discovered) {
            const isRegistered = this.repositories.some(r => r.path === repo.path);
            const statusText = isRegistered ? 'Already registered' : 'Not registered';
            const statusClass = isRegistered ? 'text-muted' : '';

            // Checkbox: disabled if already registered, checked by default if not registered
            const checkboxHtml = isRegistered ?
                '<input type="checkbox" class="repo-checkbox" disabled>' :
                '<input type="checkbox" class="repo-checkbox" data-repo-path="' + repo.path + '" checked>';

            html += '<tr>';
            html += `<td style="text-align: center;">${checkboxHtml}</td>`;
            html += `<td>${repo.path}</td>`;
            html += `<td class="text-muted">${repo.fullPath}</td>`;
            html += `<td class="${statusClass}">${statusText}</td>`;
            html += '</tr>';
        }

        html += '</tbody></table></div>';
        html += '<div class="form-actions mt-2">';
        html += '<button class="btn btn-secondary" onclick="app.navigate(\'/\')">← Cancel</button>';
        html += '<button class="btn" onclick="app.addSelectedRepositories()">Add Selected</button>';
        html += '</div>';
        html += '</div>';

        main.innerHTML = html;
    }

    selectAllRepositories(select) {
        const checkboxes = document.querySelectorAll('.repo-checkbox:not([disabled])');
        checkboxes.forEach(checkbox => {
            checkbox.checked = select;
        });
    }

    async addSelectedRepositories() {
        const checkboxes = document.querySelectorAll('.repo-checkbox:checked:not([disabled])');
        const selectedPaths = Array.from(checkboxes).map(cb => cb.getAttribute('data-repo-path'));

        if (selectedPaths.length === 0) {
            alert('No repositories selected');
            return;
        }

        try {
            // Add repositories one by one
            let successCount = 0;
            let failedRepos = [];

            for (const path of selectedPaths) {
                try {
                    const response = await fetch('/api/repositories', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({
                            path: path,
                            autoDetectWorktrees: true
                        })
                    });

                    if (response.ok) {
                        successCount++;
                    } else {
                        const error = await response.text();
                        failedRepos.push(`${path}: ${error}`);
                    }
                } catch (error) {
                    failedRepos.push(`${path}: ${error.message}`);
                }
            }

            // Show error message if there were failures
            if (failedRepos.length > 0) {
                alert(`Added ${successCount} repositories.\n\nFailed:\n${failedRepos.join('\n')}`);
            }

            // Navigate to home page
            this.navigate('/');
        } catch (error) {
            console.error('Failed to add repositories:', error);
            alert('Failed to add repositories: ' + error.message);
        }
    }

    async addRepository(path) {
        try {
            const response = await fetch('/api/repositories', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    path: path,
                    autoDetectWorktrees: true
                })
            });

            if (response.ok) {
                // Refresh the discovery page
                await this.discoverRepositories();
            } else {
                const error = await response.text();
                alert('Failed to add repository: ' + error);
            }
        } catch (error) {
            console.error('Failed to add repository:', error);
            alert('Failed to add repository: ' + error.message);
        }
    }

    async showRepositoryDetail(repoPath) {
        const main = document.getElementById('mainContent');
        main.innerHTML = '<div class="loading">Loading repository details...</div>';

        try {
            const response = await fetch(`/api/directories/${encodeURIComponent(repoPath)}`);
            if (!response.ok) {
                throw new Error('Repository not found');
            }
            const repo = await response.json();
            main.innerHTML = this.renderRepositoryDetail(repo);
        } catch (error) {
            console.error('Failed to load repository details:', error);
            main.innerHTML = `
                <div class="error">
                    <h2>Error</h2>
                    <p>Failed to load repository details: ${error.message}</p>
                    <button class="btn" onclick="app.navigate('/')">← Back to List</button>
                </div>
            `;
        }
    }

    renderRepositoryDetail(repo) {
        const hasSession = repo.activeSession !== null;
        const statusIcon = hasSession ? '●' : '○';
        const statusText = hasSession ? 'Active' : 'No active session';
        const statusClass = hasSession ? 'active' : 'idle';

        let html = '<div class="repo-detail">';

        // Header with breadcrumb
        html += '<div class="repo-detail-header">';
        html += '<h2>' + repo.path + '</h2>';
        html += '<a href="/" data-link class="btn btn-secondary">← Back to List</a>';
        html += '</div>';

        // Repository info
        html += '<div class="repo-info">';
        html += '<div class="info-row">';
        html += '<span class="info-label">Branch:</span>';
        html += '<span class="branch-name">' + (repo.gitBranch || 'unknown') + '</span>';
        html += '</div>';
        html += '<div class="info-row">';
        html += '<span class="info-label">Status:</span>';
        html += '<span class="repo-status ' + statusClass + '">' + statusIcon + ' ' + statusText + '</span>';
        html += '</div>';
        html += '<div class="info-row">';
        html += '<span class="info-label">Path:</span>';
        html += '<span class="path-value">' + repo.fullPath + '</span>';
        html += '</div>';
        html += '<div class="info-row">';
        html += '<span class="info-label">ClaudeCode History:</span>';
        html += '<span>' + (repo.hasClaudeHistory ? 'Yes' : 'No') + '</span>';
        html += '</div>';
        html += '</div>';

        // Layout toggle button
        // Priority: sessionStorage (temporary) -> config (persistent) -> default
        const sessionLayout = sessionStorage.getItem('terminalLayout');
        const configLayout = this.config?.ui?.terminalLayout || 'auto';
        const currentLayout = sessionLayout || configLayout;
        const layoutLabels = {
            'auto': '🪄 Auto',
            'horizontal': '↔ Horizontal',
            'vertical': '↕ Vertical'
        };
        html += '<div style="margin-bottom: 1rem; display: flex; justify-content: flex-end;">';
        html += '<button class="btn btn-secondary btn-sm" onclick="app.toggleTerminalLayout()">';
        html += '<span id="layoutToggleText">' + layoutLabels[currentLayout] + '</span>';
        html += '</button>';
        html += '</div>';

        // Terminals container with current layout
        html += '<div id="terminalsContainer" class="terminals-container" data-layout="' + currentLayout + '">';

        // Session terminal section
        html += '<div class="session-terminal-section" id="claudeTerminalSection">';
        html += '<div class="worktree-header">';
        html += '<h3>ClaudeCode Terminal</h3>';
        html += '<div class="button-group">';
        if (!hasSession) {
            html += '<button class="btn btn-sm" onclick="app.startSession(\'' + repo.path + '\')">▶ Start</button>';
            if (repo.hasClaudeHistory) {
                html += '<button class="btn btn-sm btn-secondary" onclick="app.resumeSession(\'' + repo.path + '\')">⏮ Resume</button>';
            }
        } else {
            html += '<button class="btn btn-danger btn-sm" onclick="app.terminateSession(\'' + repo.activeSession.id + '\')">⏹ Terminate</button>';
        }
        html += '</div>';
        html += '</div>';
        if (!hasSession) {
            html += '<div class="terminal-placeholder">';
            const message = repo.hasClaudeHistory
                ? 'Click "Start" to launch a new session or "Resume" to continue a previous session.'
                : 'Click "Start" to launch a ClaudeCode session.';
            html += '<p class="text-muted">' + message + '</p>';
            html += '</div>';
        } else {
            html += '<div id="terminal" class="terminal-container"></div>';
        }
        html += '</div>';

        // Shell terminal section
        html += '<div class="session-terminal-section">';
        html += '<div class="worktree-header">';
        html += '<h3>Shell Terminal</h3>';
        html += '<div class="button-group">';
        html += '<button class="btn btn-sm" id="shellStartBtn" onclick="app.startShellTerminal(\'' + repo.path + '\')">▶ Start Shell</button>';
        html += '</div>';
        html += '</div>';
        html += '<div id="shellTerminalContainer" class="terminal-placeholder">';
        html += '<p class="text-muted">Click "Start Shell" to launch a shell session in this directory.</p>';
        html += '</div>';
        html += '</div>';

        html += '</div>'; // Close terminals-container

        html += '</div>';

        // Setup terminal if session is active
        if (hasSession) {
            setTimeout(() => this.initializeTerminal(repo.activeSession.id), 100);
        }

        // Apply initial layout after rendering
        setTimeout(() => {
            const container = document.getElementById('terminalsContainer');
            if (container) {
                this.applyTerminalLayout(container, currentLayout);
            }
        }, 0);

        return html;
    }

    async startSession(repoPath) {
        try {
            const response = await fetch('/api/sessions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    repositoryPath: repoPath,
                    prompt: ''
                })
            });

            if (!response.ok) {
                throw new Error('Failed to start session');
            }

            const result = await response.json();
            console.log('Session started:', result);

            // Reload the page to show the new session
            this.navigate('/repo/' + repoPath, false);
        } catch (error) {
            console.error('Failed to start session:', error);
            alert('Failed to start session: ' + error.message);
        }
    }

    async resumeSession(repoPath) {
        try {
            const response = await fetch('/api/sessions/resume', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    repositoryPath: repoPath
                })
            });

            if (!response.ok) {
                throw new Error('Failed to resume session');
            }

            const result = await response.json();
            console.log('Session resumed:', result);

            // Reload the page to show the resumed session
            this.navigate('/repo/' + repoPath, false);
        } catch (error) {
            console.error('Failed to resume session:', error);
            alert('Failed to resume session: ' + error.message);
        }
    }

    async terminateSession(sessionId) {
        if (!confirm('Are you sure you want to terminate this session?')) {
            return;
        }

        try {
            // Mark that we're intentionally terminating (to prevent socket.onclose from updating UI)
            this.isTerminating = true;

            const response = await fetch(`/api/sessions/${sessionId}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                this.isTerminating = false;
                throw new Error('Failed to terminate session');
            }

            console.log('Session terminated');

            // Clean up ClaudeCode terminal WebSocket and terminal instance
            if (this.terminalCleanup) {
                this.terminalCleanup();
                this.terminalCleanup = null;
            }

            // Update only the ClaudeCode terminal section (not the whole page)
            await this.updateClaudeTerminalSection();

            this.isTerminating = false;
        } catch (error) {
            this.isTerminating = false;
            console.error('Failed to terminate session:', error);
            alert('Failed to terminate session: ' + error.message);
        }
    }

    async updateClaudeTerminalSection() {
        const claudeSection = document.getElementById('claudeTerminalSection');
        if (!claudeSection) return;

        // Get current repository path from URL
        const repoPath = window.location.pathname.substring(6); // Remove '/repo/'

        try {
            // Fetch updated repository info
            const repoResponse = await fetch(`/api/directories/${encodeURIComponent(repoPath)}`);
            if (repoResponse.ok) {
                const repo = await repoResponse.json();

                // Update ClaudeCode terminal section only
                const hasClaudeHistory = repo.hasClaudeHistory;
                claudeSection.innerHTML = `
                    <div class="worktree-header">
                        <h3>ClaudeCode Terminal</h3>
                        <div class="button-group">
                            <button class="btn btn-sm" onclick="app.startSession('${repo.path}')">▶ Start</button>
                            ${hasClaudeHistory ? '<button class="btn btn-sm btn-secondary" onclick="app.resumeSession(\'' + repo.path + '\')">⏮ Resume</button>' : ''}
                        </div>
                    </div>
                    <div class="terminal-placeholder">
                        <p class="text-muted">${hasClaudeHistory ? 'Click "Start" to launch a new session or "Resume" to continue a previous session.' : 'Click "Start" to launch a ClaudeCode session.'}</p>
                    </div>
                `;
            }
        } catch (error) {
            console.error('Failed to update ClaudeCode section:', error);
        }
    }

    toggleTerminalLayout() {
        const container = document.getElementById('terminalsContainer');
        const toggleText = document.getElementById('layoutToggleText');
        const currentLayout = container.getAttribute('data-layout') || 'auto';

        // Cycle through: auto -> horizontal -> vertical -> auto
        let newLayout;
        const layoutLabels = {
            'auto': '🪄 Auto',
            'horizontal': '↔ Horizontal',
            'vertical': '↕ Vertical'
        };

        if (currentLayout === 'auto') {
            newLayout = 'horizontal';
        } else if (currentLayout === 'horizontal') {
            newLayout = 'vertical';
        } else {
            newLayout = 'auto';
        }

        // Update container immediately for responsiveness
        container.setAttribute('data-layout', newLayout);
        toggleText.textContent = layoutLabels[newLayout];
        this.applyTerminalLayout(container, newLayout);

        // Save to sessionStorage (temporary, per-session only)
        sessionStorage.setItem('terminalLayout', newLayout);

        // Resize terminals after layout change
        setTimeout(() => {
            if (this.claudeTerminal && this.claudeFitAddon) {
                this.claudeFitAddon.fit();
            }
            if (this.shellTerminal && this.shellFitAddon) {
                this.shellFitAddon.fit();
            }
        }, 100);
    }

    applyTerminalLayout(container, layout) {
        // Remove existing layout classes
        container.classList.remove('horizontal', 'vertical', 'auto');

        if (layout === 'auto') {
            // Auto mode: use media query
            container.classList.add('auto');
        } else if (layout === 'horizontal') {
            container.classList.add('horizontal');
        } else {
            container.classList.add('vertical');
        }
    }

    async startShellTerminal(repoPath) {
        try {
            // Create a shell terminal session
            const response = await fetch('/api/terminal', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    repositoryPath: repoPath
                })
            });

            if (!response.ok) {
                throw new Error('Failed to start shell terminal');
            }

            const result = await response.json();
            const terminalId = result.data.id;

            console.log('Shell terminal started:', terminalId);

            // Replace placeholder with terminal container
            const container = document.getElementById('shellTerminalContainer');
            container.className = 'terminal-container';
            container.innerHTML = '';

            // Update button to stop button
            const startBtn = document.getElementById('shellStartBtn');
            startBtn.textContent = '⏹ Stop Shell';
            startBtn.onclick = () => this.stopShellTerminal(terminalId, repoPath);

            // Initialize the shell terminal
            this.initializeShellTerminal(terminalId, container, repoPath);
        } catch (error) {
            console.error('Failed to start shell terminal:', error);
            alert('Failed to start shell terminal: ' + error.message);
        }
    }

    async stopShellTerminal(terminalId, repoPath) {
        if (!confirm('Are you sure you want to stop the shell terminal?')) {
            return;
        }

        try {
            const response = await fetch(`/api/terminal/${terminalId}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                throw new Error('Failed to stop shell terminal');
            }

            console.log('Shell terminal stopped');

            // Cleanup shell terminal
            if (this.currentShellTerminal) {
                this.currentShellTerminal.dispose();
                this.currentShellTerminal = null;
            }
            if (this.currentShellSocket) {
                this.currentShellSocket.close();
                this.currentShellSocket = null;
            }

            // Restore placeholder
            const container = document.getElementById('shellTerminalContainer');
            container.className = 'terminal-placeholder';
            container.innerHTML = '<p class="text-muted">Click "Start Shell" to launch a shell session in this directory.</p>';

            // Update button back to start button
            const stopBtn = document.getElementById('shellStartBtn');
            stopBtn.textContent = '▶ Start Shell';
            stopBtn.onclick = () => this.startShellTerminal(repoPath);
        } catch (error) {
            console.error('Failed to stop shell terminal:', error);
            alert('Failed to stop shell terminal: ' + error.message);
        }
    }

    initializeShellTerminal(terminalId, terminalElement, repoPath) {
        if (!terminalElement) {
            console.error('Shell terminal element not found');
            return;
        }

        // Create xterm.js terminal
        const terminal = new Terminal({
            fontSize: 14,
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
                cursor: '#d4d4d4'
            }
        });

        // Load FitAddon
        const fitAddon = new FitAddon.FitAddon();
        terminal.loadAddon(fitAddon);

        // Store terminal instances for layout resize
        this.shellTerminal = terminal;
        this.shellFitAddon = fitAddon;

        // Open terminal in the container
        terminal.open(terminalElement);

        // Fit terminal to container size
        fitAddon.fit();

        // Auto-scroll tracking for shell terminal
        let shellAutoScroll = true;
        const checkShellScrollPosition = () => {
            const viewport = terminal.element.querySelector('.xterm-viewport');
            if (!viewport) return;

            const scrollTop = viewport.scrollTop;
            const scrollHeight = viewport.scrollHeight;
            const clientHeight = viewport.clientHeight;

            // User is at bottom (with 1px threshold for rounding errors)
            shellAutoScroll = (scrollTop + clientHeight >= scrollHeight - 1);
        };

        // Listen to scroll events
        const viewport = terminal.element.querySelector('.xterm-viewport');
        if (viewport) {
            viewport.addEventListener('scroll', checkShellScrollPosition);
        }

        // Connect to WebSocket
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/terminal/${terminalId}`;
        const socket = new WebSocket(wsUrl);

        socket.binaryType = 'arraybuffer';

        // Protocol constants (matching GoTTY)
        const MSG_INPUT = '1';
        const MSG_RESIZE = '3';
        const MSG_OUTPUT = '1';

        socket.onopen = () => {
            console.log('Shell WebSocket connected');

            // Send initial terminal size
            setTimeout(() => {
                const resizeMsg = MSG_RESIZE + JSON.stringify({
                    columns: terminal.cols,
                    rows: terminal.rows
                });
                console.log('Sending shell terminal size:', resizeMsg);
                socket.send(resizeMsg);
            }, 50);

            // Send data from terminal to WebSocket
            terminal.onData(data => {
                if (socket.readyState === WebSocket.OPEN) {
                    // Encode UTF-8 string as base64 (supports multibyte characters)
                    const utf8Bytes = new TextEncoder().encode(data);
                    const binaryString = Array.from(utf8Bytes, byte => String.fromCharCode(byte)).join('');
                    const encoded = btoa(binaryString);
                    socket.send(MSG_INPUT + encoded);
                }
            });

            // Send resize events
            terminal.onResize(({cols, rows}) => {
                if (socket.readyState === WebSocket.OPEN) {
                    const resizeMsg = MSG_RESIZE + JSON.stringify({
                        columns: cols,
                        rows: rows
                    });
                    socket.send(resizeMsg);
                }
            });
        };

        // Handle window resize
        const handleResize = () => {
            fitAddon.fit();
            if (shellAutoScroll) {
                terminal.scrollToBottom();
            }
        };
        window.addEventListener('resize', handleResize);

        // Handle messages
        socket.onmessage = (event) => {
            let data;
            if (event.data instanceof ArrayBuffer) {
                const decoder = new TextDecoder();
                data = decoder.decode(event.data);
            } else {
                data = event.data;
            }

            if (data.length === 0) return;

            const msgType = data[0];
            const payload = data.slice(1);

            switch (msgType) {
                case MSG_OUTPUT:
                    try {
                        const decoded = atob(payload);
                        const uint8Array = Uint8Array.from(decoded, c => c.charCodeAt(0));
                        terminal.write(uint8Array);
                        // Only auto-scroll if user was at bottom
                        if (shellAutoScroll) {
                            terminal.scrollToBottom();
                        }
                    } catch (e) {
                        console.error('Failed to decode shell output:', e);
                    }
                    break;
                default:
                    console.log('Unknown message type:', msgType);
            }
        };

        socket.onerror = (error) => {
            console.error('Shell WebSocket error:', error);
            terminal.writeln('\r\n[Connection error]\r\n');
        };

        socket.onclose = () => {
            console.log('Shell WebSocket closed');
            terminal.writeln('\r\n[Connection closed]\r\n');

            // Restore the start button after a short delay
            setTimeout(() => {
                const container = document.getElementById('shellTerminalContainer');
                if (container) {
                    container.className = 'terminal-placeholder';
                    container.innerHTML = '<p class="text-muted">Click "Start Shell" to launch a shell session in this directory.</p>';
                }

                const startBtn = document.getElementById('shellStartBtn');
                if (startBtn) {
                    startBtn.textContent = '▶ Start Shell';
                    startBtn.onclick = () => app.startShellTerminal(repoPath);
                }

                // Cleanup terminal references
                if (app.currentShellTerminal) {
                    app.currentShellTerminal.dispose();
                    app.currentShellTerminal = null;
                }
                if (app.currentShellSocket) {
                    app.currentShellSocket = null;
                }
            }, 500);
        };

        // Store references
        this.currentShellTerminal = terminal;
        this.currentShellSocket = socket;

        // Cleanup function
        const cleanup = () => {
            window.removeEventListener('resize', handleResize);
            if (this.currentShellSocket) {
                this.currentShellSocket.close();
                this.currentShellSocket = null;
            }
            if (this.currentShellTerminal) {
                this.currentShellTerminal.dispose();
                this.currentShellTerminal = null;
            }
        };

        // Store cleanup function
        this.shellTerminalCleanup = cleanup;
    }

    initializeTerminal(sessionId) {
        const terminalElement = document.getElementById('terminal');
        if (!terminalElement) {
            console.error('Terminal element not found');
            return;
        }

        // Create xterm.js terminal with minimal configuration (GoTTY style)
        // Use mostly default settings to avoid issues with TUI apps
        const terminal = new Terminal({
            fontSize: 14,
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
                cursor: '#d4d4d4'
            }
        });

        // Load FitAddon
        const fitAddon = new FitAddon.FitAddon();
        terminal.loadAddon(fitAddon);

        // Store terminal instances for layout resize
        this.claudeTerminal = terminal;
        this.claudeFitAddon = fitAddon;

        // Open terminal in the container
        terminal.open(terminalElement);

        // Fit terminal to container size
        fitAddon.fit();

        // Auto-scroll tracking for Claude terminal
        let claudeAutoScroll = true;
        const checkClaudeScrollPosition = () => {
            const viewport = terminal.element.querySelector('.xterm-viewport');
            if (!viewport) return;

            const scrollTop = viewport.scrollTop;
            const scrollHeight = viewport.scrollHeight;
            const clientHeight = viewport.clientHeight;

            // User is at bottom (with 1px threshold for rounding errors)
            claudeAutoScroll = (scrollTop + clientHeight >= scrollHeight - 1);
        };

        // Listen to scroll events
        const viewport = terminal.element.querySelector('.xterm-viewport');
        if (viewport) {
            viewport.addEventListener('scroll', checkClaudeScrollPosition);
        }

        // Connect to WebSocket
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/terminal/${sessionId}`;
        const socket = new WebSocket(wsUrl);

        socket.binaryType = 'arraybuffer';

        // Protocol constants (matching GoTTY)
        const MSG_INPUT = '1';
        const MSG_RESIZE = '3';
        const MSG_OUTPUT = '1';

        socket.onopen = () => {
            console.log('WebSocket connected for terminal');

            // Wait a brief moment for terminal to be fully initialized, then send size
            setTimeout(() => {
                const resizeMsg = MSG_RESIZE + JSON.stringify({
                    columns: terminal.cols,
                    rows: terminal.rows
                });
                console.log('Sending initial terminal size:', resizeMsg);
                socket.send(resizeMsg);
            }, 50);

            // Send data from terminal to WebSocket (with protocol prefix and base64 encoding)
            terminal.onData(data => {
                console.log('Terminal onData:', JSON.stringify(data), 'bytes:', Array.from(data).map(c => c.charCodeAt(0)));
                if (socket.readyState === WebSocket.OPEN) {
                    // Encode UTF-8 string as base64 (supports multibyte characters)
                    const utf8Bytes = new TextEncoder().encode(data);
                    const binaryString = Array.from(utf8Bytes, byte => String.fromCharCode(byte)).join('');
                    const encoded = btoa(binaryString);
                    socket.send(MSG_INPUT + encoded);
                }
            });

            // Send resize events (with protocol prefix)
            terminal.onResize(({cols, rows}) => {
                if (socket.readyState === WebSocket.OPEN) {
                    const resizeMsg = MSG_RESIZE + JSON.stringify({
                        columns: cols,
                        rows: rows
                    });
                    socket.send(resizeMsg);
                }
            });
        };

        // Handle window resize (GoTTY style)
        const handleResize = () => {
            fitAddon.fit();
            if (claudeAutoScroll) {
                terminal.scrollToBottom();
            }
        };
        window.addEventListener('resize', handleResize);

        // Handle messages with protocol parsing (GoTTY style)
        socket.onmessage = (event) => {
            let data;
            if (event.data instanceof ArrayBuffer) {
                // Convert ArrayBuffer to string
                const decoder = new TextDecoder();
                data = decoder.decode(event.data);
            } else {
                data = event.data;
            }

            if (data.length === 0) {
                return;
            }

            // Parse protocol message
            const msgType = data[0];
            const payload = data.slice(1);

            switch (msgType) {
                case MSG_OUTPUT:
                    // Decode base64 payload and write to terminal
                    try {
                        const decoded = atob(payload);
                        const uint8Array = Uint8Array.from(decoded, c => c.charCodeAt(0));
                        terminal.write(uint8Array);
                        // Only auto-scroll if user was at bottom
                        if (claudeAutoScroll) {
                            terminal.scrollToBottom();
                        }
                    } catch (e) {
                        console.error('Failed to decode output:', e);
                    }
                    break;
                default:
                    console.log('Unknown message type:', msgType);
            }
        };

        socket.onerror = (error) => {
            console.error('WebSocket error:', error);
            terminal.writeln('\r\n[Connection error]\r\n');
        };

        socket.onclose = () => {
            console.log('WebSocket closed');
            terminal.writeln('\r\n[Connection closed]\r\n');

            // Only update UI if we're not intentionally terminating (e.g., process crashed)
            if (!this.isTerminating) {
                setTimeout(() => {
                    this.updateClaudeTerminalSection();
                }, 1000);
            }
        };

        // Store references for cleanup
        this.currentTerminal = terminal;
        this.currentSocket = socket;

        // Expose helper function for DevTools testing
        window.sendToTerminal = (text) => {
            if (this.currentSocket && this.currentSocket.readyState === WebSocket.OPEN) {
                // Send text with protocol prefix and base64 encoding (supports multibyte characters)
                const utf8Bytes = new TextEncoder().encode(text + '\r');
                const binaryString = Array.from(utf8Bytes, byte => String.fromCharCode(byte)).join('');
                const encoded = btoa(binaryString);
                this.currentSocket.send(MSG_INPUT + encoded);
                console.log('Sent to terminal:', text);
                return {success: true, sent: text};
            }
            return {success: false, error: 'WebSocket not connected'};
        };

        // Expose terminal reference for DevTools
        window.terminal = terminal;

        // Cleanup on page navigation
        const cleanup = () => {
            window.removeEventListener('resize', handleResize);
            window.removeEventListener('beforeunload', cleanup);
            // Remove DevTools helpers
            delete window.sendToTerminal;
            delete window.terminal;
            if (this.currentSocket) {
                this.currentSocket.close();
                this.currentSocket = null;
            }
            if (this.currentTerminal) {
                this.currentTerminal.dispose();
                this.currentTerminal = null;
            }
        };

        // Register cleanup
        window.addEventListener('beforeunload', cleanup);

        // Store cleanup function for manual cleanup
        this.terminalCleanup = cleanup;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    async showSettings() {
        const main = document.getElementById('mainContent');
        main.innerHTML = '<div class="loading">Loading settings...</div>';

        try {
            const response = await fetch('/api/config');
            if (!response.ok) {
                throw new Error('Failed to load configuration');
            }
            const config = await response.json();
            main.innerHTML = this.renderSettings(config);
        } catch (error) {
            console.error('Failed to load settings:', error);
            main.innerHTML = `
                <div class="error">
                    <h2>Error</h2>
                    <p>Failed to load settings: ${error.message}</p>
                    <button class="btn" onclick="app.navigate('/')">← Back to List</button>
                </div>
            `;
        }
    }

    renderSettings(config) {
        let html = '<div class="settings-page">';
        html += '<div class="settings-header">';
        html += '<h2>Settings</h2>';
        html += '<button class="btn btn-secondary" onclick="window.history.back()">← Back</button>';
        html += '</div>';

        html += '<form id="settingsForm" onsubmit="app.saveSettings(event)">';

        // Basic Settings (Read-only)
        html += '<div class="settings-section">';
        html += '<h3>Basic Settings</h3>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Port (read-only)</label>';
        html += `<input type="number" value="${config.port}" readonly class="readonly-input">`;
        html += '</div>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Root Path (read-only)</label>';
        html += `<input type="text" value="${config.rootPath}" readonly class="readonly-input">`;
        html += '</div>';
        html += '</div>';

        // Worktree Settings
        html += '<div class="settings-section">';
        html += '<h3>Worktree Settings</h3>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Path Pattern</label>';
        html += `<input type="text" id="pathPattern" name="pathPattern" value="${config.worktree.pathPattern}" required>`;
        html += '<small class="text-muted">Pattern for worktree paths. Variables: &#123;repo&#125;, &#123;branch&#125;</small>';
        html += '</div>';
        html += '</div>';

        // Terminal Settings
        html += '<div class="settings-section">';
        html += '<h3>Terminal Settings</h3>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Shell</label>';
        html += `<input type="text" id="shell" name="shell" value="${config.terminal.shell}" required>`;
        html += '</div>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Rows</label>';
        html += `<input type="number" id="rows" name="rows" value="${config.terminal.rows}" min="10" max="100" required>`;
        html += '</div>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Columns</label>';
        html += `<input type="number" id="cols" name="cols" value="${config.terminal.cols}" min="40" max="200" required>`;
        html += '</div>';
        html += '</div>';

        // UI Settings
        html += '<div class="settings-section">';
        html += '<h3>UI Settings</h3>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Theme</label>';
        html += '<select id="theme" name="theme" required>';
        html += `<option value="dark" ${config.ui.theme === 'dark' ? 'selected' : ''}>Dark</option>`;
        html += `<option value="light" ${config.ui.theme === 'light' ? 'selected' : ''}>Light</option>`;
        html += '</select>';
        html += '</div>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Refresh Interval</label>';
        html += `<input type="text" id="refreshInterval" name="refreshInterval" value="${config.ui.refreshInterval}" required>`;
        html += '<small class="text-muted">Example: 5s, 1m</small>';
        html += '</div>';
        html += '<div class="form-group">';
        html += '<label class="input-label">Default Terminal Layout</label>';
        html += '<select id="terminalLayout" name="terminalLayout" required>';
        const currentTerminalLayout = config.ui.terminalLayout || 'auto';
        html += `<option value="auto" ${currentTerminalLayout === 'auto' ? 'selected' : ''}>🪄 Auto (responsive)</option>`;
        html += `<option value="horizontal" ${currentTerminalLayout === 'horizontal' ? 'selected' : ''}>↔ Horizontal</option>`;
        html += `<option value="vertical" ${currentTerminalLayout === 'vertical' ? 'selected' : ''}>↕ Vertical</option>`;
        html += '</select>';
        html += '<small class="text-muted">Default layout on page load. Can be temporarily changed using the layout button.</small>';
        html += '</div>';
        html += '</div>';

        // Registered Repositories
        html += '<div class="settings-section">';
        html += '<h3>Registered Repositories</h3>';
        if (config.repositories && config.repositories.length > 0) {
            html += '<div class="table-container"><table>';
            html += '<thead><tr><th>Path</th><th>Auto-detect Worktrees</th><th>Actions</th></tr></thead>';
            html += '<tbody>';
            for (const repo of config.repositories) {
                html += '<tr>';
                html += `<td>${repo.path}</td>`;
                html += `<td>${repo.autoDetectWorktrees ? '✓' : '✗'}</td>`;
                html += `<td><button type="button" class="btn btn-secondary btn-sm" onclick="app.removeRepository('${repo.path}')">Remove</button></td>`;
                html += '</tr>';
            }
            html += '</tbody></table></div>';
        } else {
            html += '<p class="text-muted">No repositories registered. Use Discover to add repositories.</p>';
        }
        html += '</div>';

        // Save buttons
        html += '<div class="form-actions">';
        html += '<button type="button" class="btn btn-secondary" onclick="app.navigate(\'/\')">Cancel</button>';
        html += '<button type="submit" class="btn">Save Settings</button>';
        html += '</div>';

        html += '</form>';
        html += '</div>';

        return html;
    }

    async saveSettings(event) {
        event.preventDefault();

        const form = event.target;
        const updatedConfig = {
            worktree: {
                pathPattern: form.pathPattern.value
            },
            terminal: {
                shell: form.shell.value,
                rows: parseInt(form.rows.value, 10),
                cols: parseInt(form.cols.value, 10)
            },
            ui: {
                theme: form.theme.value,
                refreshInterval: form.refreshInterval.value,
                terminalLayout: form.terminalLayout.value
            }
        };

        try {
            const response = await fetch('/api/config', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updatedConfig)
            });

            if (!response.ok) {
                const error = await response.text();
                throw new Error(error);
            }

            alert('Settings saved successfully!');
            this.navigate('/settings', false);
        } catch (error) {
            console.error('Failed to save settings:', error);
            alert('Failed to save settings: ' + error.message);
        }
    }

    async removeRepository(path) {
        if (!confirm(`Are you sure you want to remove repository "${path}"?`)) {
            return;
        }

        try {
            const response = await fetch(`/api/repositories/${encodeURIComponent(path)}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                const error = await response.text();
                throw new Error(error);
            }

            // Reload settings page
            this.navigate('/settings', false);
        } catch (error) {
            console.error('Failed to remove repository:', error);
            alert('Failed to remove repository: ' + error.message);
        }
    }

    show404() {
        const main = document.getElementById('mainContent');
        main.innerHTML = `
            <div class="empty-state">
                <h2>404 - Page Not Found</h2>
                <p>The page you're looking for doesn't exist.</p>
                <button class="btn" onclick="app.navigate('/')">← Back to Home</button>
            </div>
        `;
    }

    setupFilter() {
        const filterInput = document.getElementById('filterInput');
        if (!filterInput) return;

        filterInput.addEventListener('input', (e) => {
            const filterText = e.target.value.toLowerCase();
            const table = document.getElementById('repoTable');
            if (!table) return;

            const rows = table.querySelectorAll('tbody tr');

            rows.forEach(row => {
                // Get repository and branch text
                const repoCell = row.querySelector('.repo-path');
                const branchCell = row.querySelector('.branch-name');

                const repoText = repoCell ? repoCell.textContent.toLowerCase() : '';
                const branchText = branchCell ? branchCell.textContent.toLowerCase() : '';

                // Show row if filter matches repository or branch
                if (repoText.includes(filterText) || branchText.includes(filterText)) {
                    row.style.display = '';
                } else {
                    row.style.display = 'none';
                }
            });
        });
    }

    setupSorting() {
        const headers = document.querySelectorAll('th.sortable');
        headers.forEach(header => {
            header.style.cursor = 'pointer';
            header.addEventListener('click', () => {
                const column = header.getAttribute('data-column');

                // Toggle direction if clicking the same column
                if (this.sortColumn === column) {
                    this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
                } else {
                    this.sortColumn = column;
                    this.sortDirection = 'asc';
                }

                // Re-render the repository list with new sorting
                this.showRepositoryList();
            });
        });
    }

    sortRepositories(repos) {
        return repos.sort((a, b) => {
            let valueA, valueB;

            switch (this.sortColumn) {
                case 'status':
                    // Active > Idle > History
                    valueA = a.activeSession ? (a.activeSession.isActive ? 3 : 2) : 1;
                    valueB = b.activeSession ? (b.activeSession.isActive ? 3 : 2) : 1;
                    break;

                case 'repository':
                    valueA = a.path.toLowerCase();
                    valueB = b.path.toLowerCase();
                    break;

                case 'branch':
                    valueA = (a.gitBranch || '').toLowerCase();
                    valueB = (b.gitBranch || '').toLowerCase();
                    break;

                case 'updated':
                    // Parse date strings
                    valueA = a.lastAccessed ? new Date(a.lastAccessed).getTime() : 0;
                    valueB = b.lastAccessed ? new Date(b.lastAccessed).getTime() : 0;
                    break;

                case 'session':
                    // Active > Idle > History
                    valueA = a.activeSession ? (a.activeSession.isActive ? 3 : 2) : 1;
                    valueB = b.activeSession ? (b.activeSession.isActive ? 3 : 2) : 1;
                    break;

                default:
                    return 0;
            }

            // Apply sort direction
            if (typeof valueA === 'string') {
                return this.sortDirection === 'asc'
                    ? valueA.localeCompare(valueB)
                    : valueB.localeCompare(valueA);
            } else {
                return this.sortDirection === 'asc'
                    ? valueA - valueB
                    : valueB - valueA;
            }
        });
    }

    getRelativePathFromRoot(absolutePath) {
        // Remove rootPath prefix to get relative path
        // Example: /Users/.../testdata/aaa -> aaa
        const rootPath = this.config.rootPath;
        if (absolutePath.startsWith(rootPath + '/')) {
            return absolutePath.substring(rootPath.length + 1);
        }
        // Fallback: extract directory name
        return absolutePath.split('/').pop();
    }

    async loadWorktrees(repoPath) {
        const container = document.getElementById('worktreeList');
        if (!container) return;

        try {
            const response = await fetch(`/api/worktrees/${encodeURIComponent(repoPath)}`);
            if (!response.ok) {
                throw new Error('Failed to load worktrees');
            }

            const data = await response.json();
            const worktrees = data.data.worktrees || [];

            if (worktrees.length === 0) {
                container.innerHTML = '<p class="text-muted">No worktrees found. Click "Add Worktree" to create one.</p>';
                return;
            }

            let html = '<table class="worktree-table">';
            html += '<thead><tr><th>Branch</th><th>Path</th><th>Type</th><th>Actions</th></tr></thead>';
            html += '<tbody>';

            for (const wt of worktrees) {
                const typeLabel = wt.isMain ? 'Main' : 'Worktree';
                const typeClass = wt.isMain ? 'main' : 'worktree';

                // Calculate relative path from rootPath
                const wtRelPath = this.getRelativePathFromRoot(wt.path);

                html += '<tr>';
                html += `<td><span class="branch-name">${wt.branch || 'N/A'}</span></td>`;
                html += `<td><span class="path-value">${wt.path}</span></td>`;
                html += `<td><span class="worktree-type ${typeClass}">${typeLabel}</span></td>`;
                html += '<td class="worktree-actions">';

                // Start session button for all worktrees
                html += `<button class="btn btn-sm" onclick="app.startSession('${wtRelPath}')">▶ Start</button>`;

                // Remove button only for non-main worktrees
                if (!wt.isMain) {
                    html += ` <button class="btn btn-secondary btn-sm" onclick="app.removeWorktree('${repoPath}', '${wt.branch}')">Remove</button>`;
                }
                html += '</td>';
                html += '</tr>';
            }

            html += '</tbody></table>';
            container.innerHTML = html;
        } catch (error) {
            console.error('Failed to load worktrees:', error);
            container.innerHTML = '<p class="text-muted error">Failed to load worktrees</p>';
        }
    }

    async showWorktreeModal(repoPath) {
        // Create modal HTML
        const modalHTML = `
            <div id="worktreeModal" class="modal active">
                <div class="modal-content">
                    <div class="modal-header">
                        <h2>Add Worktree</h2>
                        <button class="modal-close" onclick="app.closeWorktreeModal()">×</button>
                    </div>
                    <form id="worktreeForm" onsubmit="app.createWorktree(event, '${repoPath}')">
                        <div class="form-group radio-group">
                            <label class="radio-label">
                                <input type="radio" name="mode" value="new" checked onchange="app.toggleWorktreeMode()">
                                <span>Create new branch</span>
                            </label>
                        </div>
                        <div id="newBranchFields" class="branch-fields">
                            <div class="form-group">
                                <label class="input-label">Branch name:</label>
                                <input type="text" id="branchName" name="branchName" placeholder="feature/my-feature" required>
                            </div>
                            <div class="form-group">
                                <label class="input-label">Base branch:</label>
                                <input type="text" id="baseBranch" name="baseBranch" value="main" placeholder="main">
                            </div>
                        </div>
                        <div class="form-group radio-group">
                            <label class="radio-label">
                                <input type="radio" name="mode" value="existing" onchange="app.toggleWorktreeMode()">
                                <span>Use existing branch</span>
                            </label>
                        </div>
                        <div id="existingBranchFields" class="branch-fields" style="display:none;">
                            <div class="form-group">
                                <label class="input-label">Select branch:</label>
                                <select id="existingBranch" name="existingBranch" disabled>
                                    <option value="">Loading branches...</option>
                                </select>
                            </div>
                        </div>
                        <div class="form-group radio-group">
                            <label class="radio-label">
                                <input type="checkbox" id="customPath" name="customPath" onchange="app.toggleCustomPath()">
                                <span>Custom path (optional)</span>
                            </label>
                        </div>
                        <div id="customPathField" class="branch-fields" style="display:none;">
                            <div class="form-group">
                                <input type="text" id="worktreePath" name="worktreePath" placeholder="relative/path/to/worktree">
                                <small class="text-muted">Default: ../{repo}-{branch}</small>
                            </div>
                        </div>
                        <div class="form-actions">
                            <button type="button" class="btn btn-secondary" onclick="app.closeWorktreeModal()">Cancel</button>
                            <button type="submit" class="btn">Create Worktree</button>
                        </div>
                    </form>
                </div>
            </div>
        `;

        // Append modal to body
        document.body.insertAdjacentHTML('beforeend', modalHTML);

        // Load branches for selection
        try {
            const response = await fetch(`/api/branches/${encodeURIComponent(repoPath)}`);
            if (response.ok) {
                const data = await response.json();
                const branches = data.data.branches || {};
                const localBranches = branches.local || [];
                const remoteBranches = branches.remote || [];

                const select = document.getElementById('existingBranch');
                select.innerHTML = '<option value="">-- Select a branch --</option>';

                // Add local branches
                if (localBranches.length > 0) {
                    const localOptGroup = document.createElement('optgroup');
                    localOptGroup.label = 'Local Branches';
                    localBranches.forEach(branch => {
                        const option = document.createElement('option');
                        option.value = branch;
                        option.textContent = branch;
                        localOptGroup.appendChild(option);
                    });
                    select.appendChild(localOptGroup);
                }

                // Add remote branches
                if (remoteBranches.length > 0) {
                    const remoteOptGroup = document.createElement('optgroup');
                    remoteOptGroup.label = 'Remote Branches';
                    remoteBranches.forEach(branch => {
                        const option = document.createElement('option');
                        option.value = branch;
                        option.textContent = branch;
                        remoteOptGroup.appendChild(option);
                    });
                    select.appendChild(remoteOptGroup);
                }
            }
        } catch (error) {
            console.error('Failed to load branches:', error);
        }
    }

    closeWorktreeModal() {
        const modal = document.getElementById('worktreeModal');
        if (modal) {
            modal.remove();
        }
    }

    toggleWorktreeMode() {
        const mode = document.querySelector('input[name="mode"]:checked').value;
        const newFields = document.getElementById('newBranchFields');
        const existingFields = document.getElementById('existingBranchFields');
        const branchInput = document.getElementById('branchName');
        const baseInput = document.getElementById('baseBranch');
        const existingSelect = document.getElementById('existingBranch');

        if (mode === 'new') {
            newFields.style.display = 'block';
            existingFields.style.display = 'none';
            branchInput.required = true;
            branchInput.disabled = false;
            baseInput.disabled = false;
            existingSelect.required = false;
            existingSelect.disabled = true;
        } else {
            newFields.style.display = 'none';
            existingFields.style.display = 'block';
            branchInput.required = false;
            branchInput.disabled = true;
            baseInput.disabled = true;
            existingSelect.required = true;
            existingSelect.disabled = false;
        }
    }

    toggleCustomPath() {
        const checkbox = document.getElementById('customPath');
        const field = document.getElementById('customPathField');
        const input = document.getElementById('worktreePath');

        if (checkbox.checked) {
            field.style.display = 'block';
            input.disabled = false;
        } else {
            field.style.display = 'none';
            input.disabled = true;
        }
    }

    async createWorktree(event, repoPath) {
        event.preventDefault();

        const form = event.target;
        const mode = form.mode.value;
        const customPath = form.customPath.checked ? form.worktreePath.value : '';

        let requestBody;
        if (mode === 'new') {
            const branchName = form.branchName.value.trim();
            const baseBranch = form.baseBranch.value.trim() || 'main';

            requestBody = {
                branch: branchName,
                baseBranch: baseBranch,
                createBranch: true,
                fromRemote: false,
                customPath: customPath
            };
        } else {
            const selectedBranch = form.existingBranch.value;
            if (!selectedBranch) {
                alert('Please select a branch');
                return;
            }

            const isRemote = selectedBranch.startsWith('origin/');
            requestBody = {
                branch: selectedBranch,
                baseBranch: '',
                createBranch: false,
                fromRemote: isRemote,
                customPath: customPath
            };
        }

        try {
            const response = await fetch(`/api/worktrees/${encodeURIComponent(repoPath)}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody)
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.message || 'Failed to create worktree');
            }

            // Close modal
            this.closeWorktreeModal();

            // Show success message
            alert('Worktree created successfully!');

            // Reload the repository list to show the new worktree
            await this.showRepositoryList();
        } catch (error) {
            console.error('Failed to create worktree:', error);
            alert('Failed to create worktree: ' + error.message);
        }
    }

    async removeWorktree(repoPath, branch) {
        if (!confirm(`Are you sure you want to remove worktree for branch "${branch}"?`)) {
            return;
        }

        try {
            const response = await fetch(`/api/worktrees/${encodeURIComponent(repoPath)}`, {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ branch })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.message || 'Failed to remove worktree');
            }

            // Reload worktrees
            await this.loadWorktrees(repoPath);
        } catch (error) {
            console.error('Failed to remove worktree:', error);
            alert('Failed to remove worktree: ' + error.message);
        }
    }
}

// Initialize app when DOM is ready
let app;
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        app = new CCListApp();
    });
} else {
    app = new CCListApp();
}
