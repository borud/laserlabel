module View.Pages.Machine exposing (sections)

import Html exposing (Html, button, details, div, form, h2, input, pre, summary, table, td, text, tr)
import Html.Attributes exposing (class, id, placeholder, value)
import Html.Events exposing (onInput, onSubmit)
import Model exposing (MachineState, Model, Msg(..), Status)
import View.Components.Form as Form
import View.Components.MachineList as MachineList
import View.Format exposing (fmt, toolName)


{-| Collapsible machine and console sections shown below the control
surface.
-}
sections : Model -> MachineState -> Html Msg
sections model m =
    div []
        [ details [ id "machine", class "card fold" ]
            [ summary [] [ text "Machine" ]
            , MachineList.view model m
            , status m
            ]
        , details [ class "card fold" ]
            [ summary [] [ text "Console" ]
            , console model m
            ]
        ]


status : MachineState -> Html Msg
status m =
    div [ class "section" ]
        [ h2 [] [ text "Status" ]
        , case m.status of
            Nothing ->
                text "…"

            Just st ->
                statusTable m st
        , div [ class "row" ]
            [ Form.button "Home" True Home
            , Form.button "Unlock" True Unlock
            , Form.dangerButton "Reset" Reset
            ]
        ]


statusTable : MachineState -> Status -> Html Msg
statusTable m st =
    let
        row k v =
            tr [] [ td [ class "key" ] [ text k ], td [] [ text v ] ]

        xyz vals =
            String.join "  " (List.map2 (\a v -> a ++ fmt v) [ "X", "Y", "Z" ] vals)

        yesNo b =
            if b then
                "yes"

            else
                "no"
    in
    table [ class "status" ]
        [ row "State" st.state
        , row "Work" (xyz st.wpos)
        , row "Machine" (xyz st.mpos)
        , row "Tool" (toolName st.tool ++ " (" ++ String.fromInt st.tool ++ ")")
        , row "Laser mode" (yesNo st.laserMode)
        , row "Homed" (yesNo m.homed ++ " (seen since connecting)")
        ]


console : Model -> MachineState -> Html Msg
console model m =
    div [ class "section" ]
        [ form [ class "row", onSubmit SendConsole ]
            [ input [ placeholder "e.g. version, $#, ?", value model.console, onInput SetConsole ] []
            , button [ class "btn" ] [ text "Send" ]
            ]
        , pre [ class "log" ] [ text (String.join "\n" (List.take 120 (List.reverse m.log))) ]
        ]
