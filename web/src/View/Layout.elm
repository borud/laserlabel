module View.Layout exposing (view)

import Html exposing (Html, a, button, div, h1, header, nav, span, text)
import Html.Attributes exposing (class, classList, href)
import Html.Events exposing (onClick)
import Model exposing (MachineState, Model, Msg(..), Tab(..))
import View.Components.Form as Form
import View.Components.JobStatus as JobStatus
import View.Pages.Control
import View.Pages.Grid
import View.Pages.Label


view : Model -> Html Msg
view model =
    div [ class "app" ]
        [ header [ class "header" ]
            [ topBar model
            , alarm model.machine
            , jobStrip model.machine
            , nav [ class "tabs" ]
                [ tab model ControlTab "Control"
                , tab model LabelTab "Label"
                , tab model GridTab "Test grid"
                ]
            ]
        , div [ class "page" ]
            [ case model.tab of
                ControlTab ->
                    View.Pages.Control.view model

                LabelTab ->
                    View.Pages.Label.view model

                GridTab ->
                    View.Pages.Grid.view model
            ]
        , case model.notice of
            Just n ->
                div [ class "toast", onClick DismissNotice ] [ text n ]

            Nothing ->
                text ""
        ]


topBar : Model -> Html Msg
topBar model =
    let
        ( name, cls, label ) =
            case model.machine of
                Nothing ->
                    ( "laserlabel", "offline", "server offline" )

                Just m ->
                    if not m.connected then
                        ( "laserlabel", "offline", "no machine" )

                    else
                        ( machineName model m
                        , Maybe.map (.state >> String.toLower) m.status |> Maybe.withDefault "offline"
                        , Maybe.map .state m.status |> Maybe.withDefault "connecting"
                        )
    in
    div [ class "topbar" ]
        [ a [ class "machine-link", href "#machine", onClick (SetTab ControlTab) ] [ h1 [] [ text (name ++ " ▾") ] ]
        , span [ class ("badge badge-" ++ cls) ] [ text label ]
        , if connected model then
            button [ class "btn btn-danger abort", onClick Abort ] [ text "ABORT" ]

          else
            text ""
        ]


machineName : Model -> MachineState -> String
machineName model m =
    model.machines
        |> List.filter (\s -> s.addr == m.addr)
        |> List.head
        |> Maybe.map .name
        |> Maybe.withDefault m.addr


alarm : Maybe MachineState -> Html Msg
alarm machine =
    case machine of
        Just m ->
            case ( m.connected, m.halt ) of
                ( True, Just reason ) ->
                    div [ class "alarm" ]
                        [ span [] [ text ("⚠ " ++ reason) ]
                        , if m.needsReset then
                            Form.dangerButton "Reset" Reset

                          else
                            Form.button "Unlock" True Unlock
                        , Form.button "Home" True Home
                        ]

                _ ->
                    text ""

        Nothing ->
            text ""


jobStrip : Maybe MachineState -> Html Msg
jobStrip machine =
    case machine of
        Just m ->
            let
                activeJob =
                    Maybe.map (\j -> j.phase == "uploading" || j.phase == "playing") m.job |> Maybe.withDefault False
            in
            if m.connected && activeJob then
                div [ class "jobstrip" ]
                    [ JobStatus.view machine
                    , Form.button "Hold" True Hold
                    , Form.button "Resume" True Resume
                    ]

            else
                text ""

        Nothing ->
            text ""


tab : Model -> Tab -> String -> Html Msg
tab model t name =
    button [ classList [ ( "tab", True ), ( "active", model.tab == t ) ], onClick (SetTab t) ] [ text name ]


connected : Model -> Bool
connected model =
    Maybe.map .connected model.machine |> Maybe.withDefault False
