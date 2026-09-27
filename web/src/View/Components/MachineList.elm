module View.Components.MachineList exposing (view)

import Html exposing (Html, button, div, form, h2, input, p, span, text)
import Html.Attributes exposing (class, placeholder, value)
import Html.Events exposing (onInput, onSubmit)
import Model exposing (MachineState, Model, Msg(..), SeenMachine)
import View.Components.Form as Form


view : Model -> MachineState -> Html Msg
view model m =
    div [ class "section" ]
        [ h2 [] [ text "Machines" ]
        , if List.isEmpty model.machines then
            p [ class "muted" ] [ text "Listening for machines on the network…" ]

          else
            div [] (List.map (row m) model.machines)
        , if m.connected then
            text ""

          else
            form [ class "row", onSubmit Connect ]
                [ input [ placeholder "other address, e.g. 192.168.1.50", value model.host, onInput SetHost ] []
                , button [ class "btn" ] [ text "Connect" ]
                ]
        , case m.error of
            Just e ->
                div [ class "error" ] [ text e ]

            Nothing ->
                text ""
        ]


row : MachineState -> SeenMachine -> Html Msg
row m seen =
    let
        current =
            m.connected && m.addr == seen.addr

        inUse =
            if seen.busy && not current then
                "in use"

            else
                ""

        note =
            String.join ", " (List.filter ((/=) "") [ seen.state, inUse ])
    in
    div [ class "machine-row" ]
        [ div [ class "machine-name" ]
            [ text seen.name
            , span [ class "muted" ] [ text (" " ++ seen.addr ++ " " ++ note) ]
            ]
        , if current then
            Form.button "Disconnect" True Disconnect

          else
            Form.button "Connect" True (ConnectTo seen.addr)
        ]
