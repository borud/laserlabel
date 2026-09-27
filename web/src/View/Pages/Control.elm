module View.Pages.Control exposing (view)

import Html exposing (Html, button, div, span, text)
import Html.Attributes exposing (class, classList, disabled)
import Html.Events exposing (onClick)
import Model exposing (MachineState, Model, Msg(..), PadMode(..), Status)
import View.Components.Form as Form
import View.Components.MachineList as MachineList
import View.Format exposing (fmt, toolName)
import View.Pages.Machine


view : Model -> Html Msg
view model =
    case model.machine of
        Nothing ->
            div [ class "card" ] [ text "Waiting for server…" ]

        Just m ->
            if not m.connected then
                div [ class "card" ] [ MachineList.view model m ]

            else
                case m.status of
                    Nothing ->
                        div [ class "card" ] [ text "Connecting…" ]

                    Just st ->
                        div [ class "surface" ]
                            [ progress m st
                            , dro m st
                            , pad model m st
                            , actions m st
                            , View.Pages.Machine.sections model m
                            ]


isIdle : Status -> Bool
isIdle st =
    st.state == "Idle" && not st.playing



-- SETUP PROGRESS


progress : MachineState -> Status -> Html Msg
progress m st =
    let
        step done label =
            span [ classList [ ( "step", True ), ( "done", done ) ] ] [ text (mark done ++ label) ]

        mark done =
            if done then
                "✓ "

            else
                "○ "
    in
    div [ class "steps" ]
        [ button [ classList [ ( "step", True ), ( "done", m.homed ) ], disabled (not (isIdle st || st.state == "Alarm")), onClick Home ]
            [ text
                (if m.homed then
                    "✓ Home"

                 else
                    "⌂ Home"
                )
            ]
        , step (st.tool == 0) "Probe"
        , step m.originSet "Origin"
        , step m.traced "Trace"
        , step m.zProbed "Z"
        ]



-- POSITION READOUT


{-| A compact readout: one cell per axis with the work position large and
the machine position small, and a zero button for X and Y.
-}
dro : MachineState -> Status -> Html Msg
dro m st =
    let
        idle =
            isIdle st

        at i xs =
            List.drop i xs |> List.head |> Maybe.withDefault 0

        axis name i zero =
            div [ class "dro-cell" ]
                [ div [ class "dro-top" ]
                    [ span [ class "dro-axis" ] [ text name ]
                    , zero
                    ]
                , div [ class "dro-work" ] [ text (fmt (at i st.wpos)) ]
                , div [ class "dro-mach" ] [ text ("m " ++ fmt (at i st.mpos)) ]
                ]

        zeroBtn name =
            button [ class "btn tiny", disabled (not idle), onClick (ZeroAxes name) ] [ text "0" ]
    in
    div [ class "card dro" ]
        [ div [ class "dro-grid" ]
            [ axis "X" 0 (zeroBtn "X")
            , axis "Y" 1 (zeroBtn "Y")
            , axis "Z" 2 (text "")
            ]
        , if m.originSet then
            text ""

          else
            div [ class "warning-line" ]
                [ span [] [ text "Origin not set this session" ]
                , button [ class "btn tiny", disabled (not idle), onClick AcceptOrigin ] [ text "Keep saved" ]
                ]
        ]



-- PAD


pad : Model -> MachineState -> Status -> Html Msg
pad model m st =
    let
        origin =
            model.padMode == MoveOrigin

        idle =
            isIdle st

        ( steps, current ) =
            if origin then
                ( [ 0.1, 0.5, 1 ], model.nudgeStep )

            else
                ( [ 0.1, 1, 10, 50 ], model.jogStep )

        enabled =
            idle && (not origin || m.originSet)

        arrow label axisName dir =
            button [ class "btn padbtn", disabled (not enabled), onClick (Pad axisName dir) ] [ text label ]

        zArrow label dir =
            button [ class "btn padbtn zbtn", disabled (not enabled || origin), onClick (Pad "Z" dir) ] [ text label ]

        stepBtn s =
            button [ classList [ ( "seg", True ), ( "selected", s == current ) ], onClick (SetPadStep s) ]
                [ text (String.fromFloat s) ]

        modeBtn mode label =
            button [ classList [ ( "seg", True ), ( "selected", model.padMode == mode ) ], onClick (SetPadMode mode) ] [ text label ]

        blank =
            span [] []

        pointerOn =
            model.pointer && st.tool == 0
    in
    div [ classList [ ( "card", True ), ( "pad", True ), ( "pad-origin", origin ) ] ]
        [ div [ class "segmented" ] [ modeBtn MoveHead "Move head", modeBtn MoveOrigin "Move origin" ]
        , div [ class "segmented" ] (List.map stepBtn steps)
        , div [ class "padgrid" ]
            [ blank, arrow "▲" "Y" 1, blank, zArrow "Z▲" 1
            , arrow "◀" "X" -1
            , span [ class "padcentre" ]
                [ text
                    (if origin then
                        "origin"

                     else
                        "head"
                    )
                ]
            , arrow "▶" "X" 1
            , blank
            , blank, arrow "▼" "Y" -1, blank, zArrow "Z▼" -1
            ]
        , if origin then
            div [ class "pad-row" ]
                [ Form.checkbox "Re-trace after each nudge" model.retrace ToggleRetrace ]

          else
            div [ class "pad-row" ]
                [ button
                    [ classList [ ( "btn", True ), ( "pointer-on", pointerOn ) ]
                    , disabled (st.tool /= 0)
                    , onClick (SetPointer (not model.pointer))
                    ]
                    [ text
                        (if pointerOn then
                            "● Laser on"

                         else
                            "○ Laser"
                        )
                    ]
                , button [ class "btn set-origin", disabled (not idle), onClick (ZeroAxes "XY") ] [ text "⊕ Set origin here" ]
                ]
        ]



-- ACTIONS


actions : MachineState -> Status -> Html Msg
actions m st =
    let
        idle =
            isIdle st

        canProbe =
            idle && m.originSet

        canGo =
            idle && m.homed

        go label target =
            button [ class "btn", disabled (not canGo), onClick (GoTo target) ] [ text label ]
    in
    div [ class "card actions-card" ]
        [ div [ class "actions" ]
            [ button [ class "btn", disabled (not canProbe), onClick (Probe True False) ] [ text "▢ Trace" ]
            , button [ class "btn", disabled (not canProbe), onClick (Probe False True) ] [ text "⤓ Probe Z" ]
            , Form.button "⌂ Home" (idle || st.state == "Alarm") Home
            ]
        , div [ class "gotopad" ]
            [ go "↖" "back-left"
            , go "Origin" "origin"
            , go "↗" "back-right"
            , go "↙" "front-left"
            , go "Centre" "center"
            , go "↘" "front-right"
            ]
        , div [ class "tool-row" ]
            [ span [] [ text ("Tool: " ++ toolName st.tool) ]
            , button [ class "btn tiny", disabled (not idle), onClick (SetTool "probe") ] [ text "→ Probe" ]
            , button [ class "btn tiny", disabled (not idle), onClick (SetTool "laser") ] [ text "→ Laser" ]
            ]
        ]
