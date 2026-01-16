#!/usr/bin/env node

/**
 * CLAI Debug MCP Server
 * 
 * Provides MCP tools for inspecting and interacting with the running CLAI TUI application
 * via its Unix socket debug interface at /tmp/clai.sock
 * 
 * Usage:
 *   node clai-mcp-server.js
 * 
 * Add to OpenCode MCP config:
 *   {
 *     "mcpServers": {
 *       "clai-debug": {
 *         "command": "node",
 *         "args": ["/path/to/clai-mcp-server.js"],
 *         "disabled": false
 *       }
 *     }
 *   }
 */

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import net from "net";
import readline from "readline";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const SOCKET_PATH = "/tmp/clai.sock";

/**
 * Send a command to the CLAI debug socket and return the response
 */
async function sendDebugCommand(command, args = null) {
  return new Promise((resolve, reject) => {
    const client = net.createConnection(SOCKET_PATH, () => {
      const message = { command, args };
      client.write(JSON.stringify(message) + "\n");
    });

    client.on("error", (err) => {
      reject(new Error(`Failed to connect to CLAI debug socket: ${err.message}`));
    });

    let responseData = "";
    const rl = readline.createInterface({ input: client, crlfDelay: Infinity });

    rl.on("line", (line) => {
      responseData += line;
      try {
        const response = JSON.parse(responseData);
        client.end();
        resolve(response);
      } catch {
        // Continue reading for more data
      }
    });

    client.on("error", (err) => {
      reject(err);
    });

    // Timeout after 5 seconds
    setTimeout(() => {
      client.destroy();
      reject(new Error("Request timed out"));
    }, 5000);
  });
}

// Create MCP server
const server = new Server(
  {
    name: "clai-debug",
    version: "1.0.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// List available tools
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: "ping",
        description: "Test connectivity to the CLAI debug server",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "inspect",
        description: "Get full UI inspection including viewport content, dimensions, pane sizes, scroll position, message count, and active pane",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "get_ui_state",
        description: "Get the current state of the running TUI - terminal dimensions, chat pane size, viewport scroll position, message count, active pane",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "inspect_styles",
        description: "Get structured viewport dimensions and state info (JSON format, ideal for programmatic use)",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "get_theme_colors",
        description: "Get the current theme colors and styling (background, foreground, dim colors for each UI element including textarea, chat bubbles, status bar, etc.)",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "switch_pane",
        description: "Switch between chat and log panes",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "get_history",
        description: "Get the conversation history/messages",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "analyze_render",
        description: "Comprehensive background coverage analysis - detects transparency gaps, compares expected vs actual colors, generates visual background maps (█ = has bg,░ = transparent)",
        inputSchema: {
          type: "object",
          properties: {},
          required: [],
        },
      },
      {
        name: "send_message",
        description: "Inject a test message into the conversation",
        inputSchema: {
          type: "object",
          properties: {
            role: {
              type: "string",
              enum: ["user", "assistant", "system"],
              description: "Message role",
            },
            content: {
              type: "string",
              description: "Message content",
            },
          },
          required: ["role", "content"],
        },
      },
      {
        name: "send_key",
        description: "Simulate a keystroke in the TUI",
        inputSchema: {
          type: "object",
          properties: {
            key: {
              type: "string",
              description: "Key to send (e.g., 'ctrl+h', 'enter', 'up', 'down', 'ctrl+c')",
            },
          },
          required: ["key"],
        },
      },
      {
        name: "type_text",
        description: "Type text into the input field character by character",
        inputSchema: {
          type: "object",
          properties: {
            text: {
              type: "string",
              description: "Text to type into the input field",
            },
          },
          required: ["text"],
        },
      },
      {
        name: "send_window_size",
        description: "Simulate a window resize event",
        inputSchema: {
          type: "object",
          properties: {
            width: {
              type: "integer",
              description: "New terminal width",
              minimum: 1,
            },
            height: {
              type: "integer",
              description: "New terminal height",
              minimum: 1,
            },
          },
          required: ["width", "height"],
        },
      },
      {
        name: "send_mouse",
        description: "Simulate a mouse event",
        inputSchema: {
          type: "object",
          properties: {
            x: {
              type: "integer",
              description: "Mouse X coordinate",
              minimum: 0,
            },
            y: {
              type: "integer",
              description: "Mouse Y coordinate",
              minimum: 0,
            },
            button: {
              type: "string",
              enum: ["left", "right", "middle", "wheel_up", "wheel_down"],
              description: "Mouse button",
            },
            action: {
              type: "string",
              enum: ["press", "release", "motion"],
              description: "Mouse action",
            },
          },
          required: ["x", "y", "button", "action"],
        },
      },
    ],
  };
});

// Handle tool calls
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    let response;

    switch (name) {
      case "ping":
        response = await sendDebugCommand("ping");
        break;

      case "inspect":
      case "get_ui_state":
        // Enable single component, offset, and limit args
        const inspectArgs = {
          ...(args || {}),
        };
        response = await sendDebugCommand("inspect", inspectArgs);
        break;

      case "inspect_styles":
        response = await sendDebugCommand("inspect_styles");
        break;

      case "get_theme_colors":
        response = await sendDebugCommand("get_theme_colors");
        break;

      case "switch_pane":
        response = await sendDebugCommand("switch_pane");
        break;

      case "get_history":
        response = await sendDebugCommand("get_history");
        break;

      case "analyze_render":
        response = await sendDebugCommand("analyze_render");
        break;

      case "send_message":
        response = await sendDebugCommand("send_message", {
          role: args.role,
          content: args.content,
        });
        break;

      case "send_key":
        response = await sendDebugCommand("send_key", {
          key: args.key,
        });
        break;

      case "type_text":
        response = await sendDebugCommand("type_text", {
          text: args.text,
        });
        break;

      case "send_window_size":
        response = await sendDebugCommand("send_window_size", {
          width: args.width,
          height: args.height,
        });
        break;

      case "send_mouse":
        response = await sendDebugCommand("send_mouse", {
          x: args.x,
          y: args.y,
          button: args.button,
          action: args.action,
        });
        break;

      default:
        return {
          content: [
            {
              type: "text",
              text: `Unknown tool: ${name}`,
            },
          ],
        };
    }

      // Format response based on success
    if (response.success) {
      let textContent;
      if (response.data) {
        // For inspect_styles, output as formatted JSON
        if (name === "inspect_styles") {
          textContent = JSON.stringify(response.data, null, 2);
        } else if (name === "inspect") {
          // For inspect command, create a readable summary
          textContent = formatInspectResponse(response.data, name);
        } else if (name === "analyze_render" && response.data.analysis) {
          // Format analyze_render response nicely
          textContent = formatAnalyzeRenderResponse(response.data);
        } else {
          // For other commands, return data as JSON or simple message
          if (typeof response.data === 'object') {
            textContent = JSON.stringify(response.data, null, 2);
          } else {
            textContent = String(response.data || "Command executed successfully");
          }
        }
      } else {
        textContent = response.message || "Command executed successfully";
      }

      return {
        content: [
          {
            type: "text",
            text: textContent,
          },
        ],
      };
    } else {
      return {
        content: [
          {
            type: "text",
            text: `Error: ${response.error || "Unknown error"}`,
          },
        ],
        isError: true,
      };
    }
  } catch (error) {
    return {
      content: [
        {
          type: "text",
          text: `Failed to execute ${name}: ${error.message}`,
        },
      ],
      isError: true,
    };
  }
});

/**
 * Format inspect response for readable output
 */
// Format compact summary and support component/offset/limit rendering for inspect
function formatInspectResponse(data, command) {
  if (command === "get_history") {
    const messages = data.messages || [];
    if (messages.length === 0) {
      return "No conversation history";
    }
    return messages
      .map((m, i) => `[${i + 1}] ${m.role}: ${m.content}`)
      .join("\n");
  }

  if (command === "switch_pane") {
    return `Active pane: ${data.active_pane}`;
  }

  // For inspect command, summarize key info
  const lines = [
    `=== CLAI UI State ===`,
    `🔄 Build ID: ${data.build_id || 'dev'}`,
    `Terminal: ${data.width}x${data.height}`,
    `Chat Pane: ${data.chat_width}x${data.chat_height}`,
    `Viewport: ${data.viewport_height} lines, offset ${data.viewport_offset}`,
    `Total Lines: ${data.total_lines}`,
    `Messages: ${data.message_count}`,
    `Active Pane: ${data.active_pane}`,
    `Show Help: ${data.show_help}`,
  ];

  return lines.join("\n");
}

/**
 * Format analyze_render response for readable output
 */
function formatAnalyzeRenderResponse(data) {
  const analysis = data.analysis;
  const lines = [
    `=== RENDER ANALYSIS ===`,
    ``,
    `Theme Background: ${data.theme_background}`,
    `Theme Foreground: ${data.theme_foreground}`,
    ``,
    `Overall Coverage: ${analysis.overallCoverage.toFixed(1)}%`,
    `Total Transparency Gaps: ${analysis.totalTransparencyGaps}`,
    ``,
    `Summary: ${analysis.summary}`,
    ``,
    `--- Component Breakdown ---`,
  ];

  for (const comp of analysis.components) {
    lines.push(``);
    lines.push(`Component: ${comp.name}`);
    lines.push(`  Background: ${comp.backgroundColor} (expected: ${comp.expectedColor})`);
    lines.push(`  Color Match: ${comp.colorMatch ? '✓' : '✗ MISMATCH'}`);
    lines.push(`  Coverage: ${comp.totalCoverage.toFixed(1)}%`);
    lines.push(`  Lines with Issues: ${comp.linesWithIssues}/${comp.totalLines}`);
    lines.push(`  Transparency Gaps: ${comp.transparencyCount}`);

    // Show visual maps if available
    if (comp.name === 'textarea' && data.textarea_maps) {
      lines.push(``);
      lines.push(`  Visual Background Map (█=has bg,░=transparent):`);
      data.textarea_maps.forEach((map, i) => {
        lines.push(`    Line ${i}: ${map}`);
      });
    }
    if (comp.name === 'viewport' && data.viewport_maps) {
      lines.push(``);
      lines.push(`  Visual Background Map (█=has bg,░=transparent):`);
      data.viewport_maps.forEach((map, i) => {
        lines.push(`    Line ${i}: ${map}`);
      });
    }
  }

  return lines.join("\n");
}

// Start the server
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("CLAI Debug MCP Server running on stdio");
}

main().catch((error) => {
  console.error("Server error:", error);
  process.exit(1);
});
