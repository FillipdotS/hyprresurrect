-- No keybinds.

hl.monitor({ output = "", mode = "1280x720@60", position = "auto", scale = 1 })

hl.config({
    animations = {
        enabled = false,
    },

    -- Deterministic tiling for the layout tests: new windows always open to
    -- the right of or below the focused one, and a split keeps its direction
    -- when the windows around it are resized.
    dwindle = {
        force_split = 2,
        preserve_split = true,
    },

    general = {
        layout = "dwindle",
        gaps_in = 0,
        gaps_out = 0,
        border_size = 1,
    },

    decoration = {
        rounding = 0,

        blur = {
            enabled = false,
        },

        shadow = {
            enabled = false,
        },
    },

    misc = {
        disable_hyprland_logo = true,
        disable_splash_rendering = true,
        force_default_wallpaper = 0,
        disable_autoreload = true,
        background_color = "rgb(000000)",
    },

    ecosystem = {
        no_update_news = true,
        no_donation_nag = true,
    },

    input = {
        follow_mouse = 0,
    },
})
