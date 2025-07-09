import { App } from "../app/app"
import { z } from "zod"
import { Bus } from "../bus"
import { Log } from "../util/log"
import path from "path"

export namespace Permission {
  const log = Log.create({ service: "permission" })

  export const Info = z
    .object({
      id: z.string(),
      sessionID: z.string(),
      title: z.string(),
      metadata: z.record(z.any()),
      time: z.object({
        created: z.number(),
      }),
    })
    .openapi({
      ref: "permission.info",
    })
  export type Info = z.infer<typeof Info>

  export const Event = {
    Updated: Bus.event("permission.updated", Info),
  }

  const state = App.state(
    "permission",
    () => {
      const pending: {
        [sessionID: string]: {
          [permissionID: string]: {
            info: Info
            resolve: () => void
            reject: (e: any) => void
          }
        }
      } = {}

      const approved: {
        [sessionID: string]: {
          [permissionID: string]: Info
        }
      } = {}

      return {
        pending,
        approved,
      }
    },
    async (state) => {
      for (const pending of Object.values(state.pending)) {
        for (const item of Object.values(pending)) {
          item.reject(new RejectedError(item.info.sessionID, item.info.id))
        }
      }
    },
  )

  export async function ask(input: {
    id: Info["id"]
    sessionID: Info["sessionID"]
    title: Info["title"]
    metadata: Info["metadata"]
  }) {
    // Check for persistent permissions first
    if (await checkPersistentPermission(input)) {
      log.info("persistent permission found", {
        sessionID: input.sessionID,
        permissionID: input.id,
      })
      return
    }
    const { pending, approved } = state()
    log.info("asking", {
      sessionID: input.sessionID,
      permissionID: input.id,
    })
    if (approved[input.sessionID]?.[input.id]) {
      log.info("previously approved", {
        sessionID: input.sessionID,
        permissionID: input.id,
      })
      return
    }
    const info: Info = {
      id: input.id,
      sessionID: input.sessionID,
      title: input.title,
      metadata: input.metadata,
      time: {
        created: Date.now(),
      },
    }
    pending[input.sessionID] = pending[input.sessionID] || {}
    return new Promise<void>((resolve, reject) => {
      pending[input.sessionID][input.id] = {
        info,
        resolve,
        reject,
      }
      // Remove auto-approval timeout - always wait for user input
      Bus.publish(Event.Updated, info)
    })
  }

  export async function respond(input: {
    sessionID: Info["sessionID"]
    permissionID: Info["id"]
    response: "once" | "always" | "reject" | "always_directory" | "always_session"
  }) {
    log.info("response", input)
    const { pending, approved } = state()
    const match = pending[input.sessionID]?.[input.permissionID]
    if (!match) return
    delete pending[input.sessionID][input.permissionID]
    if (input.response === "reject") {
      match.reject(new RejectedError(input.sessionID, input.permissionID))
      return
    }
    match.resolve()
    if (input.response === "always") {
      approved[input.sessionID] = approved[input.sessionID] || {}
      approved[input.sessionID][input.permissionID] = match.info
    } else if (input.response === "always_directory") {
      await savePersistentDirectoryPermission(match.info)
    } else if (input.response === "always_session") {
      await saveSessionPermission(match.info)
    }
  }

  export class RejectedError extends Error {
    constructor(
      public readonly sessionID: string,
      public readonly permissionID: string,
    ) {
      super(`The user rejected permission to use this functionality`)
    }
  }

  // Helper function to check persistent permissions
  async function checkPersistentPermission(input: {
    id: string
    sessionID: string
    title: string
    metadata: Record<string, any>
  }): Promise<boolean> {
    const { Config } = await import("../config/config")
    const { Storage } = await import("../storage/storage")
    const config = await Config.get()
    const app = App.info()
    const cwd = app.path.cwd

    // Check for command permissions (bash tool)
    if (input.id === "bash" && input.metadata["command"]) {
      const command = input.metadata["command"] as string
      const commandBase = command.trim().split(/\s+/)[0] // Get first word (command)

      // Check if this command base is allowed in current directory or parent directories
      const permissions = config.permissions?.directory_commands || {}

      // Check current directory and all parent directories
      let currentDir = cwd
      while (currentDir !== "/" && currentDir !== path.parse(currentDir).root) {
        const dirPermission = permissions[currentDir]
        if (dirPermission && dirPermission[commandBase]) {
          return true
        }
        currentDir = path.dirname(currentDir)
      }
    }

    // Check for session-specific file editing permissions
    if ((input.id === "edit" || input.id === "write") && input.metadata["filePath"]) {
      try {
        const sessionPermissions = await Storage.readJSON<{ file_editing?: boolean }>(
          `session/${input.sessionID}/permissions`,
        )
        if (sessionPermissions.file_editing === true) {
          return true
        }
      } catch {
        // File doesn't exist, continue
      }
    }

    return false
  }

  // Save persistent directory-based command permission
  async function savePersistentDirectoryPermission(info: Info) {
    if (info.id === "bash" && info.metadata["command"]) {
      const { Config } = await import("../config/config")
      const { Storage } = await import("../storage/storage")
      const command = info.metadata["command"] as string
      const commandBase = command.trim().split(/\s+/)[0]
      const cwd = App.info().path.cwd

      const config = await Config.get()
      const permissions = config.permissions || {}
      const directoryCommands = permissions.directory_commands || {}

      if (!directoryCommands[cwd]) {
        directoryCommands[cwd] = {}
      }
      directoryCommands[cwd][commandBase] = true

      // Save to config
      const updatedConfig = {
        ...config,
        permissions: {
          ...permissions,
          directory_commands: directoryCommands,
        },
      }

      await Storage.writeJSON("config", updatedConfig)
    }
  }

  // Save session-specific permission
  async function saveSessionPermission(info: Info) {
    if (info.id === "edit" || info.id === "write") {
      const { Storage } = await import("../storage/storage")
      let sessionPermissions: { file_editing?: boolean } = {}
      try {
        sessionPermissions = await Storage.readJSON<{ file_editing?: boolean }>(`session/${info.sessionID}/permissions`)
      } catch {
        // File doesn't exist, use default
      }
      sessionPermissions.file_editing = true
      await Storage.writeJSON(`session/${info.sessionID}/permissions`, sessionPermissions)
    }
  }
}
