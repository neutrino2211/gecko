import * as path from 'path';
import * as fs from 'fs';
import * as vscode from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;
let context: vscode.ExtensionContext | undefined;

function findLspBinary(configuredPath: string): string | undefined {
    if (configuredPath && fs.existsSync(configuredPath)) {
        return configuredPath;
    }

    const workspaceFolders = vscode.workspace.workspaceFolders;
    if (workspaceFolders) {
        for (const folder of workspaceFolders) {
            const workspacePath = path.join(folder.uri.fsPath, 'gecko-lsp');
            if (fs.existsSync(workspacePath)) {
                return workspacePath;
            }
        }
    }

    return undefined;
}

function createClient(): LanguageClient {
    const config = vscode.workspace.getConfiguration('gecko');
    const configuredPath = config.get<string>('lsp.path', '');
    const lspBinary = findLspBinary(configuredPath);
    const serverCommand = lspBinary || 'gecko-lsp';

    const serverOptions: ServerOptions = {
        command: serverCommand,
        transport: TransportKind.stdio
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'gecko' }],
        synchronize: {
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.gecko')
        }
    };

    console.log('Gecko LSP client created with:', serverCommand);

    return new LanguageClient(
        'geckoLanguageServer',
        'Gecko Language Server',
        serverOptions,
        clientOptions
    );
}

async function startClient(): Promise<void> {
    client = createClient();
    await client.start();
    console.log('Gecko LSP client started');
}

async function restartClient(): Promise<void> {
    if (client) {
        await client.stop();
        client = undefined;
    }
    await startClient();
    vscode.window.showInformationMessage('Gecko LSP restarted');
}

export function activate(ctx: vscode.ExtensionContext) {
    context = ctx;

    const config = vscode.workspace.getConfiguration('gecko');
    const lspEnabled = config.get<boolean>('lsp.enabled', true);

    if (!lspEnabled) {
        console.log('Gecko LSP is disabled');
        return;
    }

    // Register restart command
    const restartCommand = vscode.commands.registerCommand('gecko.restartLsp', async () => {
        await restartClient();
    });
    context.subscriptions.push(restartCommand);

    // Start the client
    startClient();
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
